package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type MachineService struct {
	db    *gorm.DB
	hub   *hub.Hub
	audit *AuditService
}

func NewMachineService(db *gorm.DB, wsHub *hub.Hub, audit *AuditService) *MachineService {
	return &MachineService{db: db, hub: wsHub, audit: audit}
}

type CreateMachineRequest struct {
	MachineCode string  `json:"machine_code" binding:"required"`
	GroupID     *string `json:"group_id"`
	CPUName     string  `json:"cpu_name"`
	RAMGB       int     `json:"ram_gb"`
	GPUName     string  `json:"gpu_name"`
	StorageGB   int     `json:"storage_gb"`
	OSInfo      string  `json:"os_info"`
}

type UpdateMachineRequest struct {
	GroupID   *string `json:"group_id"`
	CPUName   *string `json:"cpu_name"`
	RAMGB     *int    `json:"ram_gb"`
	GPUName   *string `json:"gpu_name"`
	StorageGB *int    `json:"storage_gb"`
	OSInfo    *string `json:"os_info"`
	Status    *string `json:"status"`
}

type HeartbeatRequest struct {
	CPUTemp float64 `json:"cpu_temp"`
	GPUTemp float64 `json:"gpu_temp"`
	IP      string  `json:"ip"`
	MAC     string  `json:"mac"`
}

type CreateMachineGroupRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Color       string  `json:"color"`
	SortOrder   int     `json:"sort_order"`
}

type UpdateMachineGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	SortOrder   *int    `json:"sort_order"`
}

type CreateMachinePriceRequest struct {
	MachineGroupID *string `json:"machine_group_id"`
	MemberGroupID  *string `json:"member_group_id"`
	PricePerHour   int64   `json:"price_per_hour" binding:"required"`
	MinDuration    int     `json:"min_duration"`
	EffectiveFrom  string  `json:"effective_from" binding:"required"`
	EffectiveTo    string  `json:"effective_to"`
}

type UpdateMachinePriceRequest struct {
	MachineGroupID *string `json:"machine_group_id"`
	MemberGroupID  *string `json:"member_group_id"`
	PricePerHour   *int64  `json:"price_per_hour"`
	MinDuration    *int    `json:"min_duration"`
	EffectiveFrom  *string `json:"effective_from"`
	EffectiveTo    *string `json:"effective_to"`
}

type CreateMachineAssetRequest struct {
	MachineID string  `json:"machine_id" binding:"required"`
	AssetType string  `json:"asset_type" binding:"required"`
	Brand     string  `json:"brand"`
	Model     string  `json:"model"`
	Serial    string  `json:"serial"`
	Status    string  `json:"status"`
	Notes     string  `json:"notes"`
}

type UpdateMachineAssetRequest struct {
	AssetType *string `json:"asset_type"`
	Brand     *string `json:"brand"`
	Model     *string `json:"model"`
	Serial    *string `json:"serial"`
	Status    *string `json:"status"`
	Notes     *string `json:"notes"`
}

func (s *MachineService) List(params pagination.Params, storeID string) (*pagination.Result, error) {
	var machines []model.Machine
	query := s.db.Where("store_id = ?", storeID)
	var total int64
	if err := query.Model(&model.Machine{}).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := pagination.Apply(query, &params).Find(&machines).Error; err != nil {
		return nil, err
	}
	return pagination.NewResult(machines, total, &params), nil
}

func (s *MachineService) GetByID(id string) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	return &machine, nil
}

func (s *MachineService) GetByCode(code string) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("machine_code = ?", code).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	return &machine, nil
}

func (s *MachineService) Create(req *CreateMachineRequest, storeID string) (*model.Machine, error) {
	var storeIDPtr *string
	if storeID != "" {
		storeIDPtr = &storeID
	}
	machine := model.Machine{
		MachineCode: req.MachineCode,
		GroupID:     req.GroupID,
		StoreID:     storeIDPtr,
		CPUName:     req.CPUName,
		RAMGB:       req.RAMGB,
		GPUName:     req.GPUName,
		StorageGB:   req.StorageGB,
		OSInfo:      req.OSInfo,
		Status:      "offline",
		IsActive:    true,
	}
	if err := s.db.Create(&machine).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine",
		EntityID:   machine.ID,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	return &machine, nil
}

func (s *MachineService) Update(id string, req *UpdateMachineRequest) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.GroupID != nil {
		updates["group_id"] = *req.GroupID
	}
	if req.CPUName != nil {
		updates["cpu_name"] = *req.CPUName
	}
	if req.RAMGB != nil {
		updates["ram_gb"] = *req.RAMGB
	}
	if req.GPUName != nil {
		updates["gpu_name"] = *req.GPUName
	}
	if req.StorageGB != nil {
		updates["storage_gb"] = *req.StorageGB
	}
	if req.OSInfo != nil {
		updates["os_info"] = *req.OSInfo
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := s.db.Model(&machine).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine",
			EntityID:   machine.ID,
			Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
		})
	}
	return &machine, nil
}

func (s *MachineService) Delete(id string) error {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine not found")
		}
		return err
	}
	if err := s.db.Delete(&machine).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine",
		EntityID:   machine.ID,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	return nil
}

func (s *MachineService) Heartbeat(id string, cpuTemp, gpuTemp float64, ip, mac string) error {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"cpu_temp":       cpuTemp,
		"gpu_temp":       gpuTemp,
		"ip_address":     ip,
		"mac_address":    mac,
		"last_heartbeat": now,
		"updated_at":     now,
	}
	if machine.Status == "offline" {
		updates["status"] = "available"
	}
	if err := s.db.Model(&machine).Updates(updates).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "heartbeat",
		EntityType: "machine",
		EntityID:   id,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	snapshot := model.MachineHardwareSnapshot{
		MachineID: id,
		CPUTemp:   cpuTemp,
		GPUTemp:   gpuTemp,
	}
	s.db.Create(&snapshot)
	return nil
}

func (s *MachineService) GetHardwareHistory(id string, params pagination.Params) (*pagination.Result, error) {
	var snapshots []model.MachineHardwareSnapshot
	query := s.db.Where("machine_id = ?", id)
	var total int64
	if err := query.Model(&model.MachineHardwareSnapshot{}).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := pagination.Apply(query, &params).Find(&snapshots).Error; err != nil {
		return nil, err
	}
	return pagination.NewResult(snapshots, total, &params), nil
}

func (s *MachineService) RemoteAction(id, action string, payload interface{}) error {
	var machine model.Machine
	if err := s.db.Select("machine_code, status").First(&machine, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine not found")
		}
		return err
	}

	event := hub.Event{
		Type: "remote:" + action,
		Data: map[string]interface{}{
			"machine_id":   id,
			"machine_code": machine.MachineCode,
			"action":       action,
			"payload":      payload,
		},
	}

	if err := s.hub.SendToMachine(machine.MachineCode, event); err != nil {
		return fmt.Errorf("send remote action: %w", err)
	}

	return nil
}

func (s *MachineService) ListGroups(storeID string) ([]model.MachineGroup, error) {
	var groups []model.MachineGroup
	if err := s.db.Where("store_id = ?", storeID).Order("sort_order asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *MachineService) CreateGroup(req *CreateMachineGroupRequest, storeID string) (*model.MachineGroup, error) {
	var storeIDPtr *string
	if storeID != "" {
		storeIDPtr = &storeID
	}
	group := model.MachineGroup{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		StoreID:     storeIDPtr,
		SortOrder:   req.SortOrder,
	}
	if err := s.db.Create(&group).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_group",
		EntityID:   group.ID,
		Metadata:   map[string]interface{}{"name": group.Name},
	})
	return &group, nil
}

func (s *MachineService) UpdateGroup(id string, req *UpdateMachineGroupRequest) (*model.MachineGroup, error) {
	var group model.MachineGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine group not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if len(updates) > 0 {
		if err := s.db.Model(&group).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine_group",
			EntityID:   group.ID,
			Metadata:   map[string]interface{}{"name": group.Name},
		})
	}
	return &group, nil
}

func (s *MachineService) DeleteGroup(id string) error {
	var group model.MachineGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine group not found")
		}
		return err
	}
	if err := s.db.Delete(&group).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_group",
		EntityID:   group.ID,
		Metadata:   map[string]interface{}{"name": group.Name},
	})
	return nil
}

func (s *MachineService) ListPrices(machineGroupID string) ([]model.MachinePrice, error) {
	var prices []model.MachinePrice
	query := s.db
	if machineGroupID != "" {
		query = query.Where("machine_group_id = ?", machineGroupID)
	}
	if err := query.Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func (s *MachineService) CreatePrice(req *CreateMachinePriceRequest) (*model.MachinePrice, error) {
	price := model.MachinePrice{
		MachineGroupID: req.MachineGroupID,
		MemberGroupID:  req.MemberGroupID,
		PricePerHour:   req.PricePerHour,
		MinDuration:    req.MinDuration,
		EffectiveFrom:  req.EffectiveFrom,
		EffectiveTo:    req.EffectiveTo,
	}
	if err := s.db.Create(&price).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_price",
		EntityID:   price.ID,
		Metadata:   map[string]interface{}{"price_per_hour": price.PricePerHour, "machine_group_id": price.MachineGroupID},
	})
	return &price, nil
}

func (s *MachineService) UpdatePrice(id string, req *UpdateMachinePriceRequest) (*model.MachinePrice, error) {
	var price model.MachinePrice
	if err := s.db.Where("id = ?", id).First(&price).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine price not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.MachineGroupID != nil {
		updates["machine_group_id"] = *req.MachineGroupID
	}
	if req.MemberGroupID != nil {
		updates["member_group_id"] = *req.MemberGroupID
	}
	if req.PricePerHour != nil {
		updates["price_per_hour"] = *req.PricePerHour
	}
	if req.MinDuration != nil {
		updates["min_duration"] = *req.MinDuration
	}
	if req.EffectiveFrom != nil {
		updates["effective_from"] = *req.EffectiveFrom
	}
	if req.EffectiveTo != nil {
		updates["effective_to"] = *req.EffectiveTo
	}
	if len(updates) > 0 {
		if err := s.db.Model(&price).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine_price",
			EntityID:   price.ID,
			Metadata:   map[string]interface{}{"price_per_hour": price.PricePerHour, "machine_group_id": price.MachineGroupID},
		})
	}
	return &price, nil
}

func (s *MachineService) DeletePrice(id string) error {
	var price model.MachinePrice
	if err := s.db.Where("id = ?", id).First(&price).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine price not found")
		}
		return err
	}
	if err := s.db.Delete(&price).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_price",
		EntityID:   price.ID,
		Metadata:   map[string]interface{}{"price_per_hour": price.PricePerHour, "machine_group_id": price.MachineGroupID},
	})
	return nil
}

func (s *MachineService) ListAssets(machineID string) ([]model.MachineAsset, error) {
	var assets []model.MachineAsset
	query := s.db
	if machineID != "" {
		query = query.Where("machine_id = ?", machineID)
	}
	if err := query.Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

func (s *MachineService) CreateAsset(req *CreateMachineAssetRequest) (*model.MachineAsset, error) {
	asset := model.MachineAsset{
		MachineID: req.MachineID,
		AssetType: req.AssetType,
		Brand:     req.Brand,
		Model:     req.Model,
		Serial:    req.Serial,
		Status:    req.Status,
		Notes:     req.Notes,
	}
	if asset.Status == "" {
		asset.Status = "good"
	}
	if err := s.db.Create(&asset).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_asset",
		EntityID:   asset.ID,
		Metadata:   map[string]interface{}{"machine_id": asset.MachineID, "asset_type": asset.AssetType},
	})
	return &asset, nil
}

func (s *MachineService) UpdateAsset(id string, req *UpdateMachineAssetRequest) (*model.MachineAsset, error) {
	var asset model.MachineAsset
	if err := s.db.Where("id = ?", id).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine asset not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.AssetType != nil {
		updates["asset_type"] = *req.AssetType
	}
	if req.Brand != nil {
		updates["brand"] = *req.Brand
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.Serial != nil {
		updates["serial"] = *req.Serial
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := s.db.Model(&asset).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine_asset",
			EntityID:   asset.ID,
			Metadata:   map[string]interface{}{"machine_id": asset.MachineID, "asset_type": asset.AssetType},
		})
	}
	return &asset, nil
}

func (s *MachineService) DeleteAsset(id string) error {
	var asset model.MachineAsset
	if err := s.db.Where("id = ?", id).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine asset not found")
		}
		return err
	}
	if err := s.db.Delete(&asset).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_asset",
		EntityID:   asset.ID,
		Metadata:   map[string]interface{}{"machine_id": asset.MachineID, "asset_type": asset.AssetType},
	})
	return nil
}
