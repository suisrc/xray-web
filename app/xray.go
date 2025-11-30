package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/infra/conf"
	conf_serial "github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/all"
)

type XrayServe struct {
	Print bool   // 是否打印配置文件
	Reset bool   // 是否重置配置文件
	Xrayc string // 配置文件

	Xconf *conf.Config   // 配置
	XrayA *core.Instance // 实例

	Start *time.Time // 启动时间
	Stopt *time.Time // 停止时间
	Exist error      // 错误
}

// ----------------------------------------------------------------------------

func (this *XrayServe) Test() {
	fmt.Println("XrayServe.Test")
}

/**
 * 判断文件是否存在
 */
func (this *XrayServe) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

/**
 * 判断Xray是否运行中
 */
func (this *XrayServe) IsRunning() bool {
	return this.XrayA != nil && this.XrayA.IsRunning()
}

// ----------------------------------------------------------------------------

/**
 * 保存Xray配置
 */
func (this *XrayServe) SaveXray() string {
	if this.Xrayc == "" {
		return "Xray配置文件为空"
	}
	bts, err := json.MarshalIndent(this.Xconf, "", "  ")
	if err != nil {
		msg := fmt.Sprintf("保存Xray配置失败: %s", err.Error())
		fmt.Println(msg)
		return msg
	}
	if err := os.WriteFile(this.Xrayc, bts, 0644); err != nil {
		msg := fmt.Sprintf("保存Xray配置失败: %s", err.Error())
		fmt.Println(msg)
		return msg
	}

	return ""
}

/**
 * 停止Xray
 */
func (this *XrayServe) StopXray() string {
	if !this.IsRunning() {
		return "Xray未启动"
	}
	stop := time.Now()
	this.Stopt = &stop
	if err := this.XrayA.Close(); err != nil {
		this.Exist = err
		msg := fmt.Sprintf("停止Xray实例失败: %s", err.Error())
		fmt.Println(msg)
		return msg
	}
	this.Exist = nil
	fmt.Printf("Xray停止成功, 停止时间: %s\n", this.Stopt.Format("2006-01-02 15:04:05"))
	return ""
}

/**
 * 重启Xray
 */
func (this *XrayServe) RestartXray(reload bool) string {
	errmsg := this.StopXray()
	if errmsg != "" {
		return errmsg
	}
	if reload {
		this.XrayA = nil
	}
	return this.StartXray()
}

// ----------------------------------------------------------------------------

/**
 * 启动Xray
 */
func (this *XrayServe) StartXray() string {
	if this.IsRunning() {
		return "Xray已启动"
	}
	// 判断是新建 or 重启
	if this.XrayA != nil {
		fmt.Println("Xray实例重启中...")
		start := time.Now()
		this.Start = &start
		if err := this.XrayA.Start(); err != nil {
			this.Exist = err
			msg := fmt.Sprintf("重启Xray实例失败: %s", err.Error())
			fmt.Println(msg)
			return msg
		}
		this.Exist = nil
		fmt.Printf("Xray重启成功, 启动时间: %s\n", this.Start.Format("2006-01-02 15:04:05"))
		return ""
	}
	// 判断 this.XrayC 文件是否存在，如果不存在，更换配置
	var err error
	cfile := this.Xrayc
	if this.Reset || !this.FileExists(cfile) {
		cfile = cfile[:strings.LastIndexByte(cfile, '.')]
		fmt.Printf("配置文件不存在: %s, 尝试使用默认配置: %s\n", this.Xrayc, cfile)
	}
	if !this.FileExists(cfile) {
		fmt.Println("默认配置文件不存在, 使用内置配置")
		this.Xconf = &conf.Config{
			InboundConfigs: []conf.InboundDetourConfig{},
		}
	} else {
		bts, err := os.ReadFile(cfile)
		if err != nil {
			msg := fmt.Sprintf("读取配置文件失败: %s", err.Error())
			fmt.Println(msg)
			return msg
		}
		// xcc, err := core.LoadConfig("json", bytes.NewReader(bts))
		// xcc, err := conf_serial.LoadJSONConfig(bytes.NewReader(bts))
		xcc, err := conf_serial.DecodeJSONConfig(bytes.NewReader(bts))
		if err != nil {
			msg := fmt.Sprintf("解析配置文件失败: %s", err.Error())
			fmt.Println(msg)
			return msg
		}
		if this.Print {
			fmt.Printf("==========================================\n")
			fmt.Printf("配置文件: %s, 配置内容: %s\n", cfile, string(bts))
			fmt.Printf("==========================================\n")
		}
		this.Xconf = xcc
	}
	xcf, err := this.Xconf.Build()
	// bts, _ := json.MarshalIndent(xcf, "", "  ")
	// fmt.Printf("==========================================\n")
	// fmt.Printf("配置文件: %s, 配置内容: %s\n", cfile, string(bts))
	// fmt.Printf("==========================================\n")

	this.XrayA, err = core.New(xcf)
	if err != nil {
		msg := fmt.Sprintf("创建Xray实例失败: %s", err.Error())
		fmt.Println(msg)
		return msg
	}
	start := time.Now()
	this.Start = &start
	if err := this.XrayA.Start(); err != nil {
		this.Exist = err
		msg := fmt.Sprintf("启动Xray实例失败: %s", err.Error())
		fmt.Println(msg)
		return msg
	}
	this.Exist = nil
	fmt.Printf("Xray启动成功, 配置文件: %s, 启动时间: %s\n", cfile, this.Start.Format("2006-01-02 15:04:05"))
	return ""
}

// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------

func (this *XrayServe) AddInbound0(cinb *core.InboundHandlerConfig) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	return core.AddInboundHandler(this.XrayA, cinb)
}

func (this *XrayServe) DelInbound0(tag string) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	ctx := context.TODO()
	mng := this.XrayA.GetFeature(inbound.ManagerType()).(inbound.Manager)
	return mng.RemoveHandler(ctx, tag)
}

func (this *XrayServe) LstInbound0() ([]any, error) {
	data := []any{}
	if this.Xconf == nil {
		return data, errors.New("未初始化配置文件")
	}
	ctx := context.TODO()
	mng := this.XrayA.GetFeature(inbound.ManagerType()).(inbound.Manager)
	for _, hdl := range mng.ListHandlers(ctx) {
		data = append(data, hdl.Tag())
	}
	return data, nil
}

func (this *XrayServe) AddOutbound0(cotb *core.OutboundHandlerConfig) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	return core.AddOutboundHandler(this.XrayA, cotb)
}

func (this *XrayServe) DelOutbound0(tag string) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	ctx := context.TODO()
	mng := this.XrayA.GetFeature(outbound.ManagerType()).(outbound.Manager)
	return mng.RemoveHandler(ctx, tag)
}

func (this *XrayServe) LstOutbound0() ([]any, error) {
	data := []any{}
	if this.Xconf == nil {
		return data, errors.New("未初始化配置文件")
	}
	ctx := context.TODO()
	mng := this.XrayA.GetFeature(outbound.ManagerType()).(outbound.Manager)
	for _, hdl := range mng.ListHandlers(ctx) {
		data = append(data, hdl.Tag())
	}
	return data, nil
}

func (this *XrayServe) AddRoute0(rule *serial.TypedMessage) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}

	rtr := this.XrayA.GetFeature(routing.RouterType()).(routing.Router)
	return rtr.AddRule(rule, true)
}

func (this *XrayServe) DelRoute0(tag string) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}

	rtr := this.XrayA.GetFeature(routing.RouterType()).(routing.Router)
	return rtr.RemoveRule(tag)
}

func (this *XrayServe) LstRoute0() ([]any, error) {
	data := []any{}
	if this.Xconf == nil {
		return data, errors.New("未初始化配置文件")
	}
	rtr := this.XrayA.GetFeature(routing.RouterType()).(*router.Router)
	val := reflect.ValueOf(rtr) // *router.Router routing.Router
	// rules := val.Elem().FieldByName("rules").Interface().([]*router.Rule)
	rule_ := unsafe.Pointer(val.Elem().FieldByName("rules").UnsafeAddr())
	rules := *(*[]*router.Rule)(rule_)
	for _, rule := range rules {
		data = append(data, rule.RuleTag)
	}
	return data, nil
}

// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------

func (this *XrayServe) AddInbound(cinb conf.InboundDetourConfig, sync bool) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	if sync {
		tag := cinb.Tag
		idx := this.FindInboundTag(tag)
		if idx >= 0 {
			return errors.New("inbound 已存在: " + tag)
		}
	}
	// 转换配置, 增加配置
	if cinc, err := cinb.Build(); err != nil {
		fmt.Println(fmt.Sprintf("inbound 转换配置文件失败: %s", err.Error()))
		return err
	} else if err := core.AddInboundHandler(this.XrayA, cinc); err != nil {
		fmt.Println(fmt.Sprintf("添加inbound 失败: %s", err.Error()))
		return err
	}
	if sync {
		// 保存配置
		this.Xconf.InboundConfigs = append(this.Xconf.InboundConfigs, cinb)
	}
	return nil
}

func (this *XrayServe) DelInbound(tag string, sync bool) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	idx := -1
	if sync {
		idx = this.FindInboundTag(tag)
		if idx < 0 {
			return errors.New("inbound 未找到: " + tag)
		}
	}
	if err := this.DelInbound0(tag); err != nil {
		fmt.Println(fmt.Sprintf("删除inbound 失败: %s", err.Error()))
		return err
	}
	if idx >= 0 {
		cons := this.Xconf.InboundConfigs
		this.Xconf.InboundConfigs = append(cons[:idx], cons[idx+1:]...)
	}
	return nil
}

func (this *XrayServe) FindInboundTag(tag string) int {
	found := -1
	for idx, ib := range this.Xconf.InboundConfigs {
		if ib.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

// ----------------------------------------------------------------------------

func (this *XrayServe) AddOutbound(cotb conf.OutboundDetourConfig, sync bool) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	if sync {
		tag := cotb.Tag
		idx := this.FindOutboundTag(tag)
		if idx >= 0 {
			return errors.New("outbound 已存在: " + tag)
		}
	}

	if ctoc, err := cotb.Build(); err != nil {
		fmt.Println(fmt.Sprintf("outbound 转换配置文件失败: %s", err.Error()))
		return err
	} else if err := core.AddOutboundHandler(this.XrayA, ctoc); err != nil {
		fmt.Println(fmt.Sprintf("添加outbound 失败: %s", err.Error()))
		return err
	}

	if sync {
		this.Xconf.OutboundConfigs = append(this.Xconf.OutboundConfigs, cotb)
	}
	return nil
}

func (this *XrayServe) DelOutbound(tag string, sync bool) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	idx := -1
	if sync {
		idx = this.FindOutboundTag(tag)
		if idx < 0 {
			return errors.New("outbound 未找到: " + tag)
		}
	}

	if err := this.DelOutbound0(tag); err != nil {
		fmt.Println(fmt.Sprintf("删除outbound 失败: %s", err.Error()))
		return err
	}

	if idx >= 0 {
		cons := this.Xconf.OutboundConfigs
		this.Xconf.OutboundConfigs = append(cons[:idx], cons[idx+1:]...)
	}
	return nil
}

func (this *XrayServe) FindOutboundTag(tag string) int {
	found := -1
	for idx, ob := range this.Xconf.OutboundConfigs {
		if ob.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

// ----------------------------------------------------------------------------

func (this *XrayServe) AddRoute(rule json.RawMessage, sync bool) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	rule_, err := conf.ParseRule(rule)
	if err != nil {
		fmt.Printf("routing 转换配置文件失败: %s", err.Error())
		return err
	}
	if sync {
		// conf.RouterRule = conf.ParseRule(json.RawMessage)
		// rul := conf.RouterRule{}; json.Unmarshal(rule, &rul)
		if rule_.RuleTag == "" {
			return errors.New("routing 未指定tag")
		}
		idx := this.FindRoutingTag(rule_.RuleTag)
		if idx >= 0 {
			return errors.New("routing 已存在: " + rule_.RuleTag)
		}
	}

	rul := &router.Config{Rule: []*router.RoutingRule{rule_}}
	if err := this.AddRoute0(serial.ToTypedMessage(rul)); err != nil {
		fmt.Println(fmt.Sprintf("routing 添加路由失败: %s", err.Error()))
		return err
	}

	if sync {
		this.Xconf.RouterConfig.RuleList = append(this.Xconf.RouterConfig.RuleList, rule)
	}
	return nil
}

func (this *XrayServe) DelRoute(tag string, sync bool) error {
	if this.Xconf == nil {
		return errors.New("未初始化配置文件")
	}
	idx := -1
	if sync {
		idx = this.FindRoutingTag(tag)
		if idx < 0 {
			return errors.New("routing  未找到: " + tag)
		}
	}

	if err := this.DelRoute0(tag); err != nil {
		fmt.Println(fmt.Sprintf("routing 删除路由失败: %s", err.Error()))
		return err
	}

	if idx >= 0 {
		cons := this.Xconf.RouterConfig.RuleList
		this.Xconf.RouterConfig.RuleList = append(cons[:idx], cons[idx+1:]...)
	}
	return nil
}

func (this *XrayServe) FindRoutingTag(tag string) int {
	found := -1
	for idx, ob := range this.Xconf.RouterConfig.RuleList {
		rule := conf.RouterRule{}
		if err := json.Unmarshal(ob, &rule); err != nil {
			continue
		}
		if rule.RuleTag == tag {
			found = idx
			break
		}
	}
	return found
}

// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------
// ----------------------------------------------------------------------------

func (this *XrayServe) GetSysStats() any {
	return ""
}
