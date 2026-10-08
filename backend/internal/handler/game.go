package handler

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
	"gorm.io/gorm"
)

type GameHandler struct {
	svc *service.GameService
}

func NewGameHandler(svc *service.GameService) *GameHandler {
	return &GameHandler{svc: svc}
}

// Service cho main.go chạy vòng nền trên cùng một đối tượng mà các route dùng.
func (h *GameHandler) Service() *service.GameService { return h.svc }

func gameError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.NotFound(c, "không tìm thấy game")
	case errors.Is(err, service.ErrGameMasterOnly), errors.Is(err, service.ErrGameCafeOnly), errors.Is(err, service.ErrGameRoleOff):
		response.Forbidden(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

// Info
// @Summary      Vai trò cập nhật game của máy chủ này
// @Tags         Games
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/games/info [get]
func (h *GameHandler) Info(c *gin.Context) {
	response.Success(c, h.svc.Info())
}

// List
// @Summary      Danh sách game và trạng thái cập nhật
// @Tags         Games
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/games [get]
func (h *GameHandler) List(c *gin.Context) {
	games, err := h.svc.List()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, games)
}

// Menu
// @Summary      Menu game cho máy trạm
// @Description  Game đang bật và có tệp chạy, kèm root (GAME_ROOT như máy trạm thấy). Hội viên và nhân viên đều gọi được.
// @Tags         Games
// @Produce      json
// @Success      200  {object}  response.Response{data=service.GameMenu}
// @Router       /api/game-menu [get]
// @Security     BearerAuth
func (h *GameHandler) Menu(c *gin.Context) {
	menu, err := h.svc.Menu()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, menu)
}

// Create
// @Summary      Thêm game để phân phối (chỉ master)
// @Description  Thư mục GAME_ROOT/<name> phải có sẵn.
// @Tags         Games
// @Accept       json
// @Produce      json
// @Success      201  {object}  response.Response
// @Router       /api/games [post]
func (h *GameHandler) Create(c *gin.Context) {
	var req service.CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	g, err := h.svc.Create(&req)
	if err != nil {
		gameError(c, err)
		return
	}
	response.Created(c, g)
}

// Update
// @Summary      Sửa game: bật/tắt, tự cập nhật, ưu tiên (master sửa được cả tên, launcher)
// @Tags         Games
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Game ID"
// @Success      200  {object}  response.Response
// @Router       /api/games/{id} [put]
func (h *GameHandler) Update(c *gin.Context) {
	var req service.UpdateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	g, err := h.svc.Update(c.Param("id"), &req)
	if err != nil {
		gameError(c, err)
		return
	}
	response.Success(c, g)
}

// Delete
// @Summary      Bỏ game khỏi danh mục (chỉ master, tệp trên ổ giữ nguyên)
// @Tags         Games
// @Param        id  path  string  true  "Game ID"
// @Success      200  {object}  response.Response
// @Router       /api/games/{id} [delete]
func (h *GameHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		gameError(c, err)
		return
	}
	response.Success(c, nil)
}

// Publish
// @Summary      Publish ngay bản hiện tại của một game (chỉ master)
// @Tags         Games
// @Param        id  path  string  true  "Game ID"
// @Success      200  {object}  response.Response
// @Router       /api/games/{id}/publish [post]
func (h *GameHandler) Publish(c *gin.Context) {
	g, err := h.svc.PublishNow(c.Request.Context(), c.Param("id"))
	if err != nil {
		gameError(c, err)
		return
	}
	response.Success(c, g)
}

// Sync
// @Summary      Tải ngay một game từ master (chỉ máy chủ quán)
// @Tags         Games
// @Param        id  path  string  true  "Game ID"
// @Success      200  {object}  response.Response
// @Router       /api/games/{id}/sync [post]
func (h *GameHandler) Sync(c *gin.Context) {
	if err := h.svc.SyncNow(c.Param("id")); err != nil {
		gameError(c, err)
		return
	}
	response.Success(c, nil)
}

// ------------------------------------------------- dành cho các quán (khoá catalog)

func (h *GameHandler) requireKey(c *gin.Context) bool {
	if !h.svc.CheckCatalogKey(c.GetHeader(service.GameCatalogKeyHeader)) {
		response.Unauthorized(c, "sai khoá catalog game")
		return false
	}
	return true
}

// Catalog
// @Summary      Catalog game của master cho các quán
// @Tags         Games
// @Produce      json
// @Param        X-Game-Key  header  string  true  "GAME_CATALOG_KEY"
// @Success      200  {object}  response.Response
// @Router       /api/game-catalog [get]
func (h *GameHandler) Catalog(c *gin.Context) {
	if !h.requireKey(c) {
		return
	}
	items, err := h.svc.Catalog()
	if err != nil {
		gameError(c, err)
		return
	}
	response.Success(c, items)
}

// Torrent
// @Summary      Tệp .torrent của bản mới nhất
// @Tags         Games
// @Produce      application/x-bittorrent
// @Param        X-Game-Key  header  string  true  "GAME_CATALOG_KEY"
// @Param        name        path    string  true  "Tên game"
// @Success      200
// @Router       /api/game-catalog/{name}/torrent [get]
func (h *GameHandler) Torrent(c *gin.Context) {
	if !h.requireKey(c) {
		return
	}
	b, err := h.svc.Torrent(c.Param("name"))
	if err != nil {
		gameError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/x-bittorrent", b)
}

// seedWriteIdle: một lần ghi tới quán được chờ tối đa bấy lâu trước khi bỏ kết nối. http.Server có
// WriteTimeout 30 giây cho CẢ phản hồi, nên một tệp 3 MB ở đường truyền 50 kB/s bị cắt giữa chừng,
// còn engine BitTorrent xin cả đoạn hàng chục MB một lần — quán WAN chậm không bao giờ tải xong.
// Không đổi WriteTimeout toàn cục (các API khác vẫn cần 30 giây); chỉ phần phát tệp được gia hạn.
var seedWriteIdle = 2 * time.Minute

// seedWriter đẩy hạn ghi của kết nối ra seedWriteIdle kể từ lúc có byte đi ra, nên chỉ một kết nối
// đứng im (quán đã mất mạng) mới bị cắt, còn kết nối chậm mà vẫn chạy thì không.
type seedWriter struct {
	http.ResponseWriter
	rc   *http.ResponseController
	next time.Time
}

func (w *seedWriter) Write(b []byte) (int, error) {
	// Gia hạn tối đa 4 lần mỗi seedWriteIdle thay vì mỗi lần ghi 32 kB.
	if now := time.Now(); !now.Before(w.next) {
		_ = w.rc.SetWriteDeadline(now.Add(seedWriteIdle)) // ResponseWriter không hỗ trợ thì giữ hạn mặc định
		w.next = now.Add(seedWriteIdle / 4)
	}
	return w.ResponseWriter.Write(b)
}

// Seed phục vụ GAME_ROOT làm webseed (BEP19): /seed/<token>/<tên game>/<tệp>.
// Hỗ trợ Range qua http.ServeContent, không liệt kê thư mục. Chỉ phát game đang
// bật và đã publish; token suy ra từ GAME_CATALOG_KEY (xem GameService.SeedToken).
func (h *GameHandler) Seed(root string) gin.HandlerFunc {
	fs := http.Dir(root)
	return func(c *gin.Context) {
		if !h.svc.CheckSeedToken(c.Param("token")) {
			c.Status(http.StatusNotFound)
			return
		}
		name := c.Param("filepath")
		parts := strings.Split(strings.TrimPrefix(name, "/"), "/")
		for _, p := range parts {
			if p == "" || p == "." || p == ".." {
				c.Status(http.StatusNotFound)
				return
			}
		}
		// Đoạn đầu là tên game, và SeedServable chỉ nhận tên đã đăng ký (không bắt
		// đầu bằng dấu chấm) — nên .vnet-gameupdate ở gốc GAME_ROOT không ra ngoài
		// được. Tệp ẩn BÊN TRONG một game (Roblox có .manifest.yaml) là một phần
		// của bản đã publish, phải phát được, không thì quán kẹt ở piece đó mãi.
		if len(parts) < 2 || !h.svc.SeedServable(parts[0]) {
			c.Status(http.StatusNotFound)
			return
		}
		f, err := fs.Open(name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil || fi.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}
		http.ServeContent(&seedWriter{ResponseWriter: c.Writer, rc: http.NewResponseController(c.Writer)},
			c.Request, filepath.Base(name), fi.ModTime(), f)
	}
}
