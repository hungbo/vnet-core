package main

import (
	"fmt"
	"html"
	"unicode/utf16"
)

// Tác vụ theo lịch: lớp bật lại dịch vụ THỨ BA, độc lập với cả dịch vụ lẫn
// giao diện.
//
// Vì sao cần thêm một lớp nữa: chế độ tự khôi phục của Service Control Manager
// CHỈ chạy khi dịch vụ chết mà không kịp báo cáo trạng thái — tức là khi bị
// kill trong Task Manager. Dừng sạch sẽ bằng `sc stop` hay Services.msc thì SCM
// coi là bình thường và KHÔNG bật lại, kể cả khi đã bật cờ
// FailureActionsOnNonCrashFailures (cờ đó chỉ nới tới "dừng với mã thoát khác
// 0"). Đó đúng là cách một người có quyền quản trị sẽ tắt dịch vụ.
//
// Task Scheduler chạy dưới SYSTEM, không phải tiến trình con của ai, nên giết
// dịch vụ và giao diện cũng không chạm được tới nó.
//
// HAI tác vụ riêng chứ không gộp làm một, có chủ đích: tác vụ theo nhịp có XML
// đơn giản gần như không thể sai, còn tác vụ theo sự kiện có bộ lọc XPath dễ
// gõ nhầm. Gộp lại thì một lỗi trong bộ lọc làm mất luôn cả lớp tin cậy.
const (
	watchdogTaskEvery = `VNET\VNET Agent Watchdog`
	watchdogTaskEvent = `VNET\VNET Agent Watchdog (sự kiện)`

	// 7036 = Service Control Manager báo một dịch vụ đổi trạng thái.
	scmServiceStateEventID = 7036
)

// taskXMLEvery dựng tác vụ chạy lúc khởi động rồi lặp mỗi phút, vô hạn.
func taskXMLEvery(exePath string) string {
	return taskXML(
		"Bật lại dịch vụ VNET nếu nó bị dừng. Chạy mỗi phút.",
		`    <BootTrigger>
      <Enabled>true</Enabled>
      <Repetition>
        <Interval>PT1M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
    </BootTrigger>`,
		exePath,
	)
}

// taskXMLOnStop dựng tác vụ kích ngay khi SCM báo dịch vụ này dừng.
//
// Lọc theo TÊN HIỂN THỊ trong EventData: sự kiện 7036 phát ra cho MỌI dịch vụ
// trên máy, không lọc thì tác vụ chạy hàng trăm lần một ngày.
func taskXMLOnStop(exePath, displayName string) string {
	query := fmt.Sprintf(
		`&lt;QueryList&gt;&lt;Query Id="0" Path="System"&gt;&lt;Select Path="System"&gt;`+
			`*[System[Provider[@Name='Service Control Manager'] and (EventID=%d)]] and `+
			`*[EventData[Data[@Name='param1']='%s'] and EventData[Data[@Name='param2']='stopped']]`+
			`&lt;/Select&gt;&lt;/Query&gt;&lt;/QueryList&gt;`,
		scmServiceStateEventID, html.EscapeString(displayName))

	return taskXML(
		"Bật lại dịch vụ VNET ngay khi nó bị dừng.",
		`    <EventTrigger>
      <Enabled>true</Enabled>
      <Subscription>`+query+`</Subscription>
    </EventTrigger>`,
		exePath,
	)
}

func taskXML(description, trigger, exePath string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.3" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>` + html.EscapeString(description) + `</Description>
  </RegistrationInfo>
  <Triggers>
` + trigger + `
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>S-1-5-18</UserId>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT2M</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + html.EscapeString(exePath) + `</Command>
      <Arguments>--ensure-service</Arguments>
    </Exec>
  </Actions>
</Task>`
}

// utf16BOMBytes chuyển chuỗi sang UTF-16LE kèm BOM.
//
// schtasks /Create /XML từ chối tệp UTF-8 trên nhiều bản Windows, và thông báo
// lỗi nó đưa ra không hề nhắc gì tới mã hoá — mất cả buổi để đoán ra.
func utf16BOMBytes(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(u)*2+2)
	out = append(out, 0xFF, 0xFE) // BOM little-endian
	for _, r := range u {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}
