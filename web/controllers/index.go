package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server"
	"ehang.io/nps/server/tool"

	"github.com/astaxie/beego"
	"github.com/xuri/excelize/v2"
)

type IndexController struct {
	BaseController
}

func (s *IndexController) Index() {
	s.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
	s.Data["data"] = server.GetDashboardData()
	s.SetInfo("dashboard")
	s.display("index/index")
}
func (s *IndexController) Help() {
	s.SetInfo("about")
	s.display("index/help")
}

func (s *IndexController) Tcp() {
	s.SetInfo("tcp")
	s.SetType("tcp")
	s.display("index/list")
}

func (s *IndexController) Udp() {
	s.SetInfo("udp")
	s.SetType("udp")
	s.display("index/list")
}

func (s *IndexController) Socks5() {
	s.SetInfo("socks5")
	s.SetType("socks5")
	s.display("index/list")
}

func (s *IndexController) Http() {
	s.SetInfo("http proxy")
	s.SetType("httpProxy")
	s.display("index/list")
}
func (s *IndexController) File() {
	s.SetInfo("file server")
	s.SetType("file")
	s.display("index/list")
}

func (s *IndexController) Secret() {
	s.SetInfo("secret")
	s.SetType("secret")
	s.display("index/list")
}
func (s *IndexController) P2p() {
	s.SetInfo("p2p")
	s.SetType("p2p")
	s.display("index/list")
}

func (s *IndexController) Host() {
	s.SetInfo("host")
	s.SetType("hostServer")
	s.display("index/list")
}

func (s *IndexController) All() {
	s.Data["menu"] = "client"
	clientId := s.getEscapeString("client_id")
	s.Data["client_id"] = clientId
	s.SetInfo("client id:" + clientId)
	s.display("index/list")
}

func (s *IndexController) GetTunnel() {
	start, length := s.GetAjaxParams()
	taskType := s.getEscapeString("type")
	clientId := s.GetIntNoErr("client_id")
	list, cnt := server.GetTunnel(start, length, taskType, clientId, s.getEscapeString("search"))
	s.AjaxTable(list, cnt, cnt, nil)
}

func (s *IndexController) Add() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["type"] = s.getEscapeString("type")
		s.Data["client_id"] = s.getEscapeString("client_id")
		s.SetInfo("add tunnel")
		s.display()
	} else {
		t := &file.Tunnel{
			Port:      s.GetIntNoErr("port"),
			ServerIp:  s.getEscapeString("server_ip"),
			Mode:      s.getEscapeString("type"),
			Target:    &file.Target{TargetStr: s.getEscapeString("target"), LocalProxy: s.GetBoolNoErr("local_proxy")},
			Id:        int(file.GetDb().JsonDb.GetTaskId()),
			Status:    true,
			Remark:    s.getEscapeString("remark"),
			Password:  s.getEscapeString("password"),
			LocalPath: s.getEscapeString("local_path"),
			StripPre:  s.getEscapeString("strip_pre"),
			Flow:      &file.Flow{},
		}
		if !tool.TestServerPort(t.Port, t.Mode) {
			s.AjaxErr("The port cannot be opened because it may has been occupied or is no longer allowed.")
		}
		var err error
		if t.Client, err = file.GetDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
			s.AjaxErr(err.Error())
		}
		if t.Client.MaxTunnelNum != 0 && t.Client.GetTunnelNum() >= t.Client.MaxTunnelNum {
			s.AjaxErr("The number of tunnels exceeds the limit")
		}
		if err := file.GetDb().NewTask(t); err != nil {
			s.AjaxErr(err.Error())
		}
		if err := server.AddTask(t); err != nil {
			s.AjaxErr(err.Error())
		} else {
			s.AjaxOk("add success")
		}
	}
}
func (s *IndexController) GetOneTunnel() {
	id := s.GetIntNoErr("id")
	data := make(map[string]interface{})
	if t, err := file.GetDb().GetTask(id); err != nil {
		data["code"] = 0
	} else {
		data["code"] = 1
		data["data"] = t
	}
	s.Data["json"] = data
	s.ServeJSON()
}
func (s *IndexController) Edit() {
	id := s.GetIntNoErr("id")
	if s.Ctx.Request.Method == "GET" {
		if t, err := file.GetDb().GetTask(id); err != nil {
			s.error()
		} else {
			s.Data["t"] = t
		}
		s.SetInfo("edit tunnel")
		s.display()
	} else {
		if t, err := file.GetDb().GetTask(id); err != nil {
			s.error()
		} else {
			if client, err := file.GetDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
				s.AjaxErr("modified error,the client is not exist")
				return
			} else {
				t.Client = client
			}
			if s.GetIntNoErr("port") != t.Port {
				if !tool.TestServerPort(s.GetIntNoErr("port"), t.Mode) {
					s.AjaxErr("The port cannot be opened because it may has been occupied or is no longer allowed.")
					return
				}
				t.Port = s.GetIntNoErr("port")
			}
			t.ServerIp = s.getEscapeString("server_ip")
			t.Mode = s.getEscapeString("type")
			t.Target = &file.Target{TargetStr: s.getEscapeString("target")}
			t.Password = s.getEscapeString("password")
			t.Id = id
			t.LocalPath = s.getEscapeString("local_path")
			t.StripPre = s.getEscapeString("strip_pre")
			t.Remark = s.getEscapeString("remark")
			t.Target.LocalProxy = s.GetBoolNoErr("local_proxy")
			file.GetDb().UpdateTask(t)
			server.StopServer(t.Id)
			server.StartTask(t.Id)
		}
		s.AjaxOk("modified success")
	}
}

//批量修改tcp/udp隧道的客户端ID
func (s *IndexController) BatchEdit() {
	if !s.isAdmin() {
		s.AjaxErr("没有权限执行此操作")
		return
	}
	idsStr := strings.TrimSpace(s.getEscapeString("ids"))
	if idsStr == "" {
		s.AjaxErr("请先勾选要修改的隧道")
		return
	}
	clientId := s.GetIntNoErr("client_id")
	if clientId <= 0 {
		s.AjaxErr("请填写要修改的客户端 ID")
		return
	}
	c, err := file.GetDb().GetClient(clientId)
	if err != nil {
		s.AjaxErr(fmt.Sprintf("客户端 %d 不存在", clientId))
		return
	}
	var success, failed int
	var reasons []string
	for _, item := range strings.Split(idsStr, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			failed++
			continue
		}
		t, err := file.GetDb().GetTask(id)
		if err != nil {
			failed++
			continue
		}
		if t.Mode != "tcp" && t.Mode != "udp" {
			failed++
			reasons = append(reasons, fmt.Sprintf("ID %d 不是 TCP/UDP 隧道", id))
			continue
		}
		if t.Client == nil {
			failed++
			reasons = append(reasons, fmt.Sprintf("ID %d 的客户端不存在", id))
			continue
		}
		if t.Client.Id == clientId {
			success++
			continue
		}
		t.Client = c
		file.GetDb().UpdateTask(t)
		if t.Status {
			server.StopServer(t.Id)
			server.StartTask(t.Id)
		}
		success++
	}
	msg := fmt.Sprintf("批量修改完成：成功 %d 个，失败 %d 个", success, failed)
	if failed > 0 {
		if len(reasons) > 3 {
			reasons = reasons[:3]
		}
		msg += "（" + strings.Join(reasons, "；") + "）"
	}
	if success == 0 {
		s.AjaxErr(msg)
		return
	}
	s.AjaxOk(msg)
}

//批量删除tcp/udp隧道
func (s *IndexController) BatchDel() {
	if !s.isAdmin() {
		s.AjaxErr("没有权限执行此操作")
		return
	}
	idsStr := strings.TrimSpace(s.getEscapeString("ids"))
	if idsStr == "" {
		s.AjaxErr("请先勾选要删除的隧道")
		return
	}
	var success, failed int
	var reasons []string
	for _, item := range strings.Split(idsStr, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			failed++
			continue
		}
		t, err := file.GetDb().GetTask(id)
		if err != nil {
			failed++
			reasons = append(reasons, fmt.Sprintf("ID %d 不存在", id))
			continue
		}
		if t.Mode != "tcp" && t.Mode != "udp" {
			failed++
			reasons = append(reasons, fmt.Sprintf("ID %d 不是 TCP/UDP 隧道", id))
			continue
		}
		if err := server.DelTask(id); err != nil {
			failed++
			reasons = append(reasons, fmt.Sprintf("ID %d 删除失败", id))
			continue
		}
		success++
	}
	msg := fmt.Sprintf("批量删除完成：成功 %d 个，失败 %d 个", success, failed)
	if failed > 0 {
		if len(reasons) > 3 {
			reasons = reasons[:3]
		}
		msg += "（" + strings.Join(reasons, "；") + "）"
	}
	if success == 0 {
		s.AjaxErr(msg)
		return
	}
	s.AjaxOk(msg)
}

//批量添加隧道页面
func (s *IndexController) BatchAdd() {
	if !s.isAdmin() {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/", 302)
		return
	}
	s.Data["menu"] = "tcp"
	s.SetInfo("batch add")
	s.display("index/batchadd")
}

//下载批量添加的 Excel 模板
func (s *IndexController) BatchTemplate() {
	if !s.isAdmin() {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/", 302)
		return
	}
	xlsx := excelize.NewFile()
	defer xlsx.Close()
	sheet := "隧道"
	xlsx.SetSheetName("Sheet1", sheet)
	headers := []string{"类型(tcp/udp)", "客户端ID", "备注", "服务端端口", "目标 IP:端口", "服务端IP(可选)"}
	headStyle, err := xlsx.NewStyle(`{"font":{"bold":true,"color":"#FFFFFF"},"fill":{"type":"pattern","color":["#1A7BB9"],"pattern":1},"alignment":{"horizontal":"center","vertical":"center"},"border":[{"type":"left","color":"#DDE3E8"},{"type":"right","color":"#DDE3E8"},{"type":"top","color":"#DDE3E8"},{"type":"bottom","color":"#DDE3E8"}]}`)
	if err != nil {
		headStyle = 0
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		xlsx.SetCellValue(sheet, cell, h)
		if headStyle != 0 {
			xlsx.SetCellStyle(sheet, cell, cell, headStyle)
		}
	}
	widths := []float64{14, 12, 20, 14, 26, 20}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		xlsx.SetColWidth(sheet, col, col, w)
	}
	xlsx.SetRowHeight(sheet, 1, 24)
	_ = xlsx.SetPanes(sheet, `{"freeze":true,"split":false,"x_split":0,"y_split":1,"top_left_cell":"A2","active_pane":"bottomLeft","state":"frozen"}`)

	info := "填写说明"
	xlsx.NewSheet(info)
	tips := []string{
		"批量添加 TCP / UDP 隧道 · 填写说明",
		"",
		"一、使用步骤",
		"1. 打开“隧道”工作表，从第 2 行开始一行填一条隧道；第 1 行表头不要删除、不要改名、不要调换列顺序。",
		"2. 保存为 .xlsx 格式（不要保存成 .csv 或 .xls）。",
		"3. 回到 NPS 管理页，点击“批量添加”，选择“批量上传”，选中文件后点击“确认添加”。",
		"4. 页面会提示成功/失败条数；失败的记录不会创建，修正后可重新上传。",
		"",
		"二、每一列要输入什么（按列顺序 A-F 填写）",
		"A 类型(tcp/udp)：必填。只能填 tcp 或 udp 这两个字母，大小写都可以；不要填 http、socks5、p2p 等。示例：tcp",
		"B 客户端ID：必填。只能填数字（正整数），且必须是“客户端列表”里已经存在的 ID，不能填客户端名称。示例：1",
		"C 备注：可留空。可填中文、英文、数字和常用符号，建议 30 字以内。示例：公司远程桌面",
		"D 服务端端口：必填。只能填 1-65535 的整数，不能带空格或其他字符；不能与本服务器上已有隧道端口重复（tcp 和 udp 可以使用同一个数字）。示例：3389",
		"E 目标 IP:端口：必填。格式是 IP或域名 + 英文冒号(:) + 端口，冒号必须是英文冒号。示例：192.168.1.10:3389、example.com:80",
		"   需要多个目标时，在同一单元格内用 Alt+Enter 换行，每行一个目标。",
		"F 服务端IP(可选)：可留空。留空表示使用服务器全部网卡；也可以填服务器上的某个 IP。示例：0.0.0.0",
		"",
		"三、正确填写示例（详细版见“示例”工作表）",
		"tcp ｜ 1 ｜ 公司远程桌面 ｜ 3389 ｜ 192.168.1.10:3389 ｜ 留空",
		"udp ｜ 2 ｜ 家庭游戏加速 ｜ 5000 ｜ 192.168.1.20:5000 ｜ 留空",
		"tcp ｜ 3 ｜ 网站测试 ｜ 8080 ｜ example.com:80 ｜ 0.0.0.0",
		"",
		"四、常见错误（会导致该行添加失败）",
		"类型填成 http/socks5、客户端ID不存在、端口已被占用、端口不是数字、目标为空、目标里用了中文冒号（：）、备注里误填了端口等。",
		"空行会被自动忽略；单次最多导入 500 条；单条失败不影响其他行。",
	}
	for i, line := range tips {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		xlsx.SetCellValue(info, cell, line)
		if i == 0 {
			if headStyle != 0 {
				xlsx.SetCellStyle(info, cell, cell, headStyle)
			}
		}
	}
	_ = xlsx.SetColWidth(info, "A", "A", 120)

	demo := "示例"
	xlsx.NewSheet(demo)
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		xlsx.SetCellValue(demo, cell, h)
		if headStyle != 0 {
			xlsx.SetCellStyle(demo, cell, cell, headStyle)
		}
	}
	demoRows := [][]interface{}{
		{"tcp", 1, "公司远程桌面", 3389, "192.168.1.10:3389", ""},
		{"udp", 2, "家庭游戏加速", 5000, "192.168.1.20:5000", ""},
		{"tcp", 3, "网站测试", 8080, "example.com:80", "0.0.0.0"},
	}
	for i, row := range demoRows {
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			xlsx.SetCellValue(demo, cell, v)
		}
	}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		xlsx.SetColWidth(demo, col, col, w)
	}
	xlsx.SetRowHeight(demo, 1, 24)
	noteCell, _ := excelize.CoordinatesToCellName(1, len(demoRows)+2)
	xlsx.SetCellValue(demo, noteCell, "说明：以上仅为示例，请不要直接上传；请把内容复制到“隧道”工作表中按需修改。“隧道”工作表第 2 行起才是导入数据。")
	xlsx.MergeCell(demo, noteCell, "F"+strconv.Itoa(len(demoRows)+2))

	xlsx.SetActiveSheet(0)
	buf, err := xlsx.WriteToBuffer()
	if err != nil {
		s.AjaxErr("生成模板失败: " + err.Error())
		return
	}
	s.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	s.Ctx.Output.Header("Content-Disposition", "attachment; filename=nps-tunnel-template.xlsx")
	s.Ctx.Output.Body(buf.Bytes())
}

//批量上传 Excel 并添加隧道
func (s *IndexController) BatchUpload() {
	if !s.isAdmin() {
		s.AjaxErr("没有权限执行此操作")
		return
	}
	if s.Ctx.Request.Method != "POST" {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/index/batchadd", 302)
		return
	}
	f, _, err := s.GetFile("file")
	if err != nil {
		s.AjaxErr("请选择要上传的 Excel 文件")
		return
	}
	defer f.Close()
	xlsx, err := excelize.OpenReader(f)
	if err != nil {
		s.AjaxErr("Excel 文件解析失败，请下载模板填写后上传")
		return
	}
	defer xlsx.Close()
	if xlsx.SheetCount == 0 {
		s.AjaxErr("Excel 文件中没有工作表")
		return
	}
	sheet := "隧道"
	if xlsx.GetSheetIndex(sheet) < 0 {
		sheet = xlsx.GetSheetName(0)
	}
	rows, err := xlsx.GetRows(sheet)
	if err != nil {
		s.AjaxErr("读取 Excel 内容失败: " + err.Error())
		return
	}
	if len(rows) < 2 {
		s.AjaxErr("Excel 中没有数据，请从第 2 行开始填写")
		return
	}
	if len(rows) > 501 {
		s.AjaxErr("单次最多导入 500 条，请拆分文件后上传")
		return
	}
	var success, failed int
	var reasons []string
	imported := make(map[string]bool)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNo := i + 1
		mode := strings.ToLower(strings.TrimSpace(batchCell(row, 0)))
		clientIdStr := strings.TrimSpace(batchCell(row, 1))
		remark := strings.TrimSpace(batchCell(row, 2))
		portStr := strings.TrimSpace(batchCell(row, 3))
		target := strings.TrimSpace(batchCell(row, 4))
		serverIp := strings.TrimSpace(batchCell(row, 5))
		if mode == "" && clientIdStr == "" && portStr == "" && target == "" {
			continue
		}
		if mode != "tcp" && mode != "udp" {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 类型只能填 tcp 或 udp", rowNo))
			continue
		}
		clientId, err := strconv.Atoi(clientIdStr)
		if err != nil || clientId <= 0 {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 客户端ID无效", rowNo))
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 服务端端口无效", rowNo))
			continue
		}
		if target == "" {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 目标 IP:端口不能为空", rowNo))
			continue
		}
		c, err := file.GetDb().GetClient(clientId)
		if err != nil {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 客户端 %d 不存在", rowNo, clientId))
			continue
		}
		key := mode + "-" + strconv.Itoa(port)
		if imported[key] {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 端口 %d 在文件中重复", rowNo, port))
			continue
		}
		if batchPortExists(mode, port) {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 端口 %d 已被占用", rowNo, port))
			continue
		}
		if !tool.TestServerPort(port, mode) {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 端口 %d 不可用", rowNo, port))
			continue
		}
		t := &file.Tunnel{
			Port:     port,
			ServerIp: serverIp,
			Mode:     mode,
			Status:   true,
			Client:   c,
			Remark:   remark,
			Target:   &file.Target{TargetStr: target},
			Id:       int(file.GetDb().JsonDb.GetTaskId()),
			Flow:     &file.Flow{},
		}
		if err := file.GetDb().NewTask(t); err != nil {
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 保存失败: %s", rowNo, err.Error()))
			continue
		}
		if err := server.AddTask(t); err != nil {
			file.GetDb().DelTask(t.Id)
			failed++
			reasons = append(reasons, fmt.Sprintf("第%d行 启动失败: %s", rowNo, err.Error()))
			continue
		}
		imported[key] = true
		success++
	}
	msg := fmt.Sprintf("批量添加完成：成功 %d 条，失败 %d 条", success, failed)
	if failed > 0 {
		if len(reasons) > 5 {
			reasons = reasons[:5]
		}
		msg += "（" + strings.Join(reasons, "；") + "）"
	}
	if success == 0 {
		s.AjaxErr(msg)
		return
	}
	s.AjaxOk(msg)
}

//读取 Excel 行中指定列，越界返回空
func batchCell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return row[index]
}

//判断服务器上是否已有同协议同端口的隧道
func batchPortExists(mode string, port int) bool {
	exists := false
	file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v, ok := value.(*file.Tunnel)
		if !ok || v.Port != port || port == 0 {
			return true
		}
		if v.Mode == mode {
			exists = true
			return false
		}
		return true
	})
	return exists
}

func (s *IndexController) Stop() {
	id := s.GetIntNoErr("id")
	if err := server.StopServer(id); err != nil {
		s.AjaxErr("stop error")
	}
	s.AjaxOk("stop success")
}

func (s *IndexController) Del() {
	id := s.GetIntNoErr("id")
	if err := server.DelTask(id); err != nil {
		s.AjaxErr("delete error")
	}
	s.AjaxOk("delete success")
}

func (s *IndexController) Start() {
	id := s.GetIntNoErr("id")
	if err := server.StartTask(id); err != nil {
		s.AjaxErr("start error")
	}
	s.AjaxOk("start success")
}

func (s *IndexController) HostList() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["client_id"] = s.getEscapeString("client_id")
		s.Data["menu"] = "host"
		s.SetInfo("host list")
		s.display("index/hlist")
	} else {
		start, length := s.GetAjaxParams()
		clientId := s.GetIntNoErr("client_id")
		list, cnt := file.GetDb().GetHost(start, length, clientId, s.getEscapeString("search"))
		s.AjaxTable(list, cnt, cnt, nil)
	}
}

func (s *IndexController) GetHost() {
	if s.Ctx.Request.Method == "POST" {
		data := make(map[string]interface{})
		if h, err := file.GetDb().GetHostById(s.GetIntNoErr("id")); err != nil {
			data["code"] = 0
		} else {
			data["data"] = h
			data["code"] = 1
		}
		s.Data["json"] = data
		s.ServeJSON()
	}
}

func (s *IndexController) DelHost() {
	id := s.GetIntNoErr("id")
	if err := file.GetDb().DelHost(id); err != nil {
		s.AjaxErr("delete error")
	}
	s.AjaxOk("delete success")
}

func (s *IndexController) AddHost() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["client_id"] = s.getEscapeString("client_id")
		s.Data["menu"] = "host"
		s.SetInfo("add host")
		s.display("index/hadd")
	} else {
		h := &file.Host{
			Id:           int(file.GetDb().JsonDb.GetHostId()),
			Host:         s.getEscapeString("host"),
			Target:       &file.Target{TargetStr: s.getEscapeString("target"), LocalProxy: s.GetBoolNoErr("local_proxy")},
			HeaderChange: s.getEscapeString("header"),
			HostChange:   s.getEscapeString("hostchange"),
			Remark:       s.getEscapeString("remark"),
			Location:     s.getEscapeString("location"),
			Flow:         &file.Flow{},
			Scheme:       s.getEscapeString("scheme"),
			KeyFilePath:  s.getEscapeString("key_file_path"),
			CertFilePath: s.getEscapeString("cert_file_path"),
		}
		var err error
		if h.Client, err = file.GetDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
			s.AjaxErr("add error the client can not be found")
		}
		if err := file.GetDb().NewHost(h); err != nil {
			s.AjaxErr("add fail" + err.Error())
		}
		s.AjaxOk("add success")
	}
}

func (s *IndexController) EditHost() {
	id := s.GetIntNoErr("id")
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "host"
		if h, err := file.GetDb().GetHostById(id); err != nil {
			s.error()
		} else {
			s.Data["h"] = h
		}
		s.SetInfo("edit")
		s.display("index/hedit")
	} else {
		if h, err := file.GetDb().GetHostById(id); err != nil {
			s.error()
		} else {
			if h.Host != s.getEscapeString("host") {
				tmpHost := new(file.Host)
				tmpHost.Host = s.getEscapeString("host")
				tmpHost.Location = s.getEscapeString("location")
				tmpHost.Scheme = s.getEscapeString("scheme")
				if file.GetDb().IsHostExist(tmpHost) {
					s.AjaxErr("host has exist")
					return
				}
			}
			if client, err := file.GetDb().GetClient(s.GetIntNoErr("client_id")); err != nil {
				s.AjaxErr("modified error,the client is not exist")
			} else {
				h.Client = client
			}
			h.Host = s.getEscapeString("host")
			h.Target = &file.Target{TargetStr: s.getEscapeString("target")}
			h.HeaderChange = s.getEscapeString("header")
			h.HostChange = s.getEscapeString("hostchange")
			h.Remark = s.getEscapeString("remark")
			h.Location = s.getEscapeString("location")
			h.Scheme = s.getEscapeString("scheme")
			h.KeyFilePath = s.getEscapeString("key_file_path")
			h.CertFilePath = s.getEscapeString("cert_file_path")
			h.Target.LocalProxy = s.GetBoolNoErr("local_proxy")
			file.GetDb().JsonDb.StoreHostToJsonFile()
		}
		s.AjaxOk("modified success")
	}
}

//备份恢复页面
func (s *IndexController) Backup() {
	if !s.isAdmin() {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/", 302)
		return
	}
	s.Data["menu"] = "backup"
	s.SetInfo("backup")
	s.display("index/backup")
}

func (s *IndexController) backupFilePaths() map[string]string {
	db := file.GetDb().JsonDb
	return map[string]string{
		"clients.json": db.ClientFilePath,
		"tasks.json":   db.TaskFilePath,
		"hosts.json":   db.HostFilePath,
	}
}

//下载配置备份包到本地
func (s *IndexController) Download() {
	if !s.isAdmin() {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/", 302)
		return
	}
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, path := range s.backupFilePaths() {
		data, err := ioutil.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			s.AjaxErr("读取配置文件失败: " + err.Error())
			return
		}
		if os.IsNotExist(err) {
			data = []byte("")
		}
		w, err := zw.Create(name)
		if err != nil {
			s.AjaxErr("生成备份文件失败: " + err.Error())
			return
		}
		_, _ = w.Write(data)
	}
	if w, err := zw.Create("backup-info.txt"); err == nil {
		_, _ = w.Write([]byte("NPS configuration backup\ncreated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n"))
	}
	if err := zw.Close(); err != nil {
		s.AjaxErr("生成备份文件失败: " + err.Error())
		return
	}
	fileName := "nps-backup-" + time.Now().Format("20060102-150405") + ".zip"
	s.Ctx.Output.Header("Content-Type", "application/zip")
	s.Ctx.Output.Header("Content-Disposition", "attachment; filename="+fileName)
	s.Ctx.Output.Body(buf.Bytes())
}

//上传备份包并恢复配置
func (s *IndexController) Restore() {
	if !s.isAdmin() {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/", 302)
		return
	}
	if s.Ctx.Request.Method != "POST" {
		s.Redirect(beego.AppConfig.String("web_base_url")+"/index/backup", 302)
		return
	}
	f, _, err := s.GetFile("file")
	if err != nil {
		s.AjaxErr("请选择要恢复的备份文件")
		return
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		s.AjaxErr("读取上传文件失败: " + err.Error())
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		s.AjaxErr("备份文件格式错误，请上传本系统导出的 zip 备份包")
		return
	}
	paths := s.backupFilePaths()
	contents := make(map[string][]byte)
	for _, zf := range zr.File {
		name := filepath.Base(zf.Name)
		if _, ok := paths[name]; !ok {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			s.AjaxErr("读取备份文件失败: " + err.Error())
			return
		}
		b, err := ioutil.ReadAll(rc)
		rc.Close()
		if err != nil {
			s.AjaxErr("读取备份文件失败: " + err.Error())
			return
		}
		if err := validateConfigFile(b); err != nil {
			s.AjaxErr("配置文件 " + name + " 内容无效: " + err.Error())
			return
		}
		contents[name] = b
	}
	if len(contents) == 0 {
		s.AjaxErr("备份文件中未找到 clients.json、tasks.json 或 hosts.json")
		return
	}
	for name, b := range contents {
		if err := writeFileAtomic(paths[name], b); err != nil {
			s.AjaxErr("写入配置文件失败: " + err.Error())
			return
		}
	}
	server.ReloadAll()
	s.AjaxOk("恢复成功，配置已生效")
}

//校验配置文件内容，每条记录为一行 json
func validateConfigFile(b []byte) error {
	for _, v := range strings.Split(string(b), "\n"+common.CONN_DATA_SEQ) {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		var tmp interface{}
		if err := json.Unmarshal([]byte(v), &tmp); err != nil {
			return err
		}
	}
	return nil
}

//先写临时文件，再替换原文件
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
