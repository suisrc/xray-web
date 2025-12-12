package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
)

/**
 * 定义处理函数
 */
type HandlerFunc = func(ac string, ww http.ResponseWriter, rr *http.Request)

/**
 * 定义处理对象
 */
type Worker struct {
	Token string
	Route map[string]HandlerFunc
	Serve XrayServe
}

/**
 * 创建处理对象
 */
func NewHandler() *Worker {
	worker := &Worker{}
	worker.Route = map[string]HandlerFunc{
		"healthz": worker.healthz,
	}
	return worker
}

// ----------------------------------------------------------------------------

/**
 * 定义响应结构体
 */
type Result struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	ErrCode string `json:"errcode,omitempty"`
	Message string `json:"message,omitempty"`
	TraceId string `json:"traceid,omitempty"`
}

/**
 * 请求结构体
 */
type ReqBody struct {
	Atyp string `json:"type"`
	Data string `json:"data"`
}

/**
 * 响应结果
 */
func Response(rr *http.Request, ww http.ResponseWriter, resp *Result) {
	ww.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(ww).Encode(resp)
}

/**
 * HTTP处理
 */
func (this *Worker) ServeHTTP(ww http.ResponseWriter, rr *http.Request) {
	// 需要验证令牌
	if this.Token != "" {
		if token := rr.Header.Get("Authorization"); token == "Token "+this.Token {
			// pass
		} else {
			resp := Result{ErrCode: "invalid_token", Message: "无效的令牌"}
			Response(rr, ww, &resp)
			return
		}
	}
	// 处理 action
	action := rr.URL.Query().Get("action")
	if action == "" {
		rpath := rr.URL.Path
		if len(rpath) > 0 {
			rpath = rpath[1:] // 删除前缀 '/'
		}
		action = rpath
	}
	if action == "" {
		resp := Result{ErrCode: "empty_action", Message: "空的操作"}
		Response(rr, ww, &resp)
		return
	}
	if rr.Method != http.MethodPost && action != "healthz" {
		// 只有 healthz 允许 GET 请求
		resp := Result{ErrCode: "invalid_method", Message: "无效的请求方法"}
		Response(rr, ww, &resp)
		return
	} else if strings.HasPrefix(action, "xray.") {
		this.xrayz(action, ww, rr)
	} else if handle, ok := this.Route[action]; ok {
		handle(action, ww, rr)
	} else {
		resp := Result{ErrCode: "invalid_action", Message: "无效的操作: " + action}
		Response(rr, ww, &resp)
	}
}

// ----------------------------------------------------------------------------

func (this *Worker) healthz(ac string, ww http.ResponseWriter, rr *http.Request) {
	nowf := time.Now().Format("2006-01-02 15:04:05")
	resp := Result{Success: true, Data: nowf}
	Response(rr, ww, &resp)
}

// ----------------------------------------------------------------------------

type IOboundCoreConfig struct {
	Inbound  core.InboundHandlerConfig  `json:"inbound"`
	Outbound core.OutboundHandlerConfig `json:"outbound"`
}

type IOboundConfConfig struct {
	Inbound  conf.InboundDetourConfig  `json:"inbound"`
	Outbound conf.OutboundDetourConfig `json:"outbound"`
}

/**
 *
 * Xray 处理
 *
 * 参数同 配置 文件
 * xray.app.proxyman.conf.AddInbound
 * xray.app.proxyman.conf.DelInbound
 * xray.app.proxyman.conf.LstInbound
 * xray.app.proxyman.conf.AddOutbound
 * xray.app.proxyman.conf.DelOutbound
 * xray.app.proxyman.conf.LstOutbound
 * xray.app.proxyman.conf.AddRoute
 * xray.app.proxyman.conf.DelRoute
 * xray.app.proxyman.conf.LstRoute
 *
 * 参数同 grpc API
 * xray.app.proxyman.core.AddInbound
 * xray.app.proxyman.core.DelInbound
 * xray.app.proxyman.core.LstInbound
 * xray.app.proxyman.core.AddOutbound
 * xray.app.proxyman.core.DelOutbound
 * xray.app.proxyman.core.LstOutbound
 * xray.app.proxyman.core.AddRoute
 * xray.app.proxyman.core.DelRoute
 * xray.app.proxyman.core.LstRoute
 *
 * 兼容 in & out
 * xray.app.proxyman.conf.AddIObound
 * xray.app.proxyman.conf.DelIObound
 * xray.app.proxyman.core.AddIObound
 * xray.app.proxyman.core.DelIObound
 *
 * 参数同 stats API
 * xray.app.proxyman.conf.GetSysStats
 * xray.app.proxyman.core.GetSysStats
 * xray.app.proxyman.conf.GetStats
 * xray.app.proxyman.core.GetStats
 * xray.app.proxyman.conf.LstStats
 * xray.app.proxyman.core.LstStats
 *
 */
func (this *Worker) xrayz(ac string, ww http.ResponseWriter, rr *http.Request) {
	var resp *Result = nil

	switch ac {
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.AddInbound":
		// 添加入站
		xcc := conf.InboundDetourConfig{}
		if err := json.NewDecoder(rr.Body).Decode(&xcc); err != nil {
			resp = &Result{ErrCode: "invalid_json", Message: "无效的 JSON: " + err.Error()}
		} else if err := this.Serve.AddInbound(xcc, false); err != nil {
			resp = &Result{ErrCode: "error_add_inbound", Message: "错误: " + err.Error()}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.core.AddInbound":
		// 添加入站
		xcc := core.InboundHandlerConfig{}
		if err := json.NewDecoder(rr.Body).Decode(&xcc); err != nil {
			resp = &Result{ErrCode: "invalid_json", Message: "无效的 JSON: " + err.Error()}
		} else if err := this.Serve.AddInbound0(&xcc); err != nil {
			resp = &Result{ErrCode: "error_add_inbound", Message: "错误: " + err.Error()}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.DelInbound", "xray.app.proxyman.core.DelInbound":
		// 删除入站
		if tag := rr.URL.Query().Get("tag"); tag == "" {
			resp = &Result{ErrCode: "invalid_tag", Message: "无效的 tag"}
		} else if err := this.Serve.DelInbound(tag, false); err != nil {
			resp = &Result{ErrCode: "error_del_inbound", Message: "错误: " + err.Error()}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.LstInbound", "xray.app.proxyman.core.LstInbound":
		// 列出入站
		data, _ := this.Serve.LstInbound0()
		resp = &Result{Success: true, Data: data}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.AddOutbound":
		// 添加出站
		xcc := conf.OutboundDetourConfig{}
		if err := json.NewDecoder(rr.Body).Decode(&xcc); err != nil {
			resp = &Result{ErrCode: "invalid_json", Message: "无效的 JSON: " + err.Error()}
		} else if err := this.Serve.AddOutbound(xcc, false); err != nil {
			resp = &Result{ErrCode: "error_add_outbound", Message: "错误: " + err.Error()}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.core.AddOutbound":
		// 添加出站
		xcc := core.OutboundHandlerConfig{}
		if err := json.NewDecoder(rr.Body).Decode(&xcc); err != nil {
			resp = &Result{ErrCode: "invalid_json", Message: "无效的 JSON: " + err.Error()}
		} else if err := this.Serve.AddOutbound0(&xcc); err != nil {
			resp = &Result{ErrCode: "error_add_outbound", Message: "错误: " + err.Error()}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.DelOutbound", "xray.app.proxyman.core.DelOutbound":
		// 删除出站
		if tag := rr.URL.Query().Get("tag"); tag == "" {
			resp = &Result{ErrCode: "invalid_tag", Message: "无效的 tag"}
		} else if err := this.Serve.DelOutbound(tag, false); err != nil {
			resp = &Result{ErrCode: "error_del_outbound"}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.LstOutbound", "xray.app.proxyman.core.LstOutbound":
		// 列出出站
		data, _ := this.Serve.LstOutbound0()
		resp = &Result{Success: true, Data: data}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.AddRoute", "xray.app.proxyman.core.AddRoute":
		// 添加路由
		if bts, err := io.ReadAll(rr.Body); err != nil {
			resp = &Result{ErrCode: "invalid_data", Message: "无效的数据: " + err.Error()}
		} else {
			var raw json.RawMessage = bts
			if err := this.Serve.AddRoute(raw, false); err != nil {
				resp = &Result{ErrCode: "error_add_route", Message: "错误: " + err.Error()}
			} else {
				resp = &Result{Success: true}
			}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.DelRoute", "xray.app.proxyman.core.DelRoute":
		// 删除路由
		if tag := rr.URL.Query().Get("tag"); tag == "" {
			resp = &Result{ErrCode: "invalid_tag", Message: "无效的 tag"}
		} else if err := this.Serve.DelRoute(tag, false); err != nil {
			resp = &Result{ErrCode: "error_del_route", Message: "错误: " + err.Error()}
		} else {
			resp = &Result{Success: true}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.LstRoute", "xray.app.proxyman.core.LstRoute":
		// 列路由
		data, _ := this.Serve.LstRoute0()
		resp = &Result{Success: true, Data: data}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.AddIObound":
		// 添加入站 & 添加出站
		xcc := IOboundConfConfig{}
		if err := json.NewDecoder(rr.Body).Decode(&xcc); err != nil {
			resp = &Result{ErrCode: "invalid_json", Message: "无效的 JSON: " + err.Error()}
		} else if xcc.Inbound.Tag != xcc.Outbound.Tag {
			rmsg := fmt.Sprintf("错误的 tag: %s(in) != %s(out)", xcc.Inbound.Tag, xcc.Outbound.Tag)
			resp = &Result{ErrCode: "no_same_tag", Message: rmsg}
		} else {
			// 构建路由
			tag := xcc.Inbound.Tag
			rule_ := &router.RoutingRule{
				RuleTag:    tag,
				TargetTag:  &router.RoutingRule_Tag{Tag: tag},
				InboundTag: []string{tag},
			}
			rul := &router.Config{Rule: []*router.RoutingRule{rule_}}
			// 部署配置
			rule := serial.ToTypedMessage(rul)
			err1 := this.Serve.AddOutbound(xcc.Outbound, false)
			err2 := this.Serve.AddInbound(xcc.Inbound, false)
			err3 := this.Serve.AddRoute0(rule)
			if err1 != nil || err2 != nil || err3 != nil {
				rmsg := fmt.Sprintf("错误: %s(tag) -> %v(out), %v(in), %v(route)", tag, err1, err2, err3)
				resp = &Result{ErrCode: "error_del_iobound", Message: rmsg}
			} else {
				resp = &Result{Success: true}
			}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.core.AddIObound":
		// 添加入站 & 添加出站
		xcc := IOboundCoreConfig{}
		if err := json.NewDecoder(rr.Body).Decode(&xcc); err != nil {
			resp = &Result{ErrCode: "invalid_json", Message: "无效的 JSON: " + err.Error()}
		} else if xcc.Inbound.Tag != xcc.Outbound.Tag {
			rmsg := fmt.Sprintf("错误的 tag: %s(in) != %s(out)", xcc.Inbound.Tag, xcc.Outbound.Tag)
			resp = &Result{ErrCode: "no_same_tag", Message: rmsg}
		} else {
			// 构建路由
			tag := xcc.Inbound.Tag
			rule_ := &router.RoutingRule{
				RuleTag:    tag,
				TargetTag:  &router.RoutingRule_Tag{Tag: tag},
				InboundTag: []string{tag},
			}
			rul := &router.Config{Rule: []*router.RoutingRule{rule_}}
			// 部署配置
			rule := serial.ToTypedMessage(rul)
			err1 := this.Serve.AddOutbound0(&xcc.Outbound)
			err2 := this.Serve.AddInbound0(&xcc.Inbound)
			err3 := this.Serve.AddRoute0(rule)
			if err1 != nil || err2 != nil || err3 != nil {
				rmsg := fmt.Sprintf("错误: %s(tag) -> %v(out), %v(in), %v(route)", tag, err1, err2, err3)
				resp = &Result{ErrCode: "error_del_iobound", Message: rmsg}
			} else {
				resp = &Result{Success: true}
			}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.DelIObound", "xray.app.proxyman.core.DelIObound":
		// 删除入站 & 删除出站
		if tag := rr.URL.Query().Get("tag"); tag == "" {
			resp = &Result{ErrCode: "invalid_tag", Message: "无效的 tag"}
		} else {
			err1 := this.Serve.DelOutbound(tag, false)
			err2 := this.Serve.DelInbound(tag, false)
			err3 := this.Serve.DelRoute(tag, false)
			if err1 != nil || err2 != nil || err3 != nil {
				rmsg := fmt.Sprintf("错误: %s(tag) -> %v(out), %v(in), %v(route)", tag, err1, err2, err3)
				resp = &Result{ErrCode: "error_del_iobound", Message: rmsg}
			} else {
				resp = &Result{Success: true}
			}
		}
	// -------------------------------------------------------------------------------
	case "xray.app.proxyman.conf.GetSysStats", "xray.app.proxyman.core.GetSysStats":
		resp = &Result{Success: true, Data: this.Serve.GetSysStats()}
	}
	// -------------------------------------------------------------------------------
	if resp == nil {
		resp = &Result{ErrCode: "invalid_xray", Message: "无效的操作: " + ac}
	}
	// 处理返回值
	Response(rr, ww, resp)
}

// ----------------------------------------------------------------------------
