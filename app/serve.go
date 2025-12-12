package app

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/xtls/xray-core/core"
)

//go:embed version
var version string

func Version() string {
	return strings.TrimSpace(version)
}

/**
 * 启动HTTP服务
 */
func Serve() {
	var (
		addr   string
		port   int
		offset int
		config string
		ver    bool
	)
	handler := NewHandler()
	// ------------------------------------------------------------------------
	flag.StringVar(&addr, "addr", "127.0.0.1", "HTTP服务地址")
	flag.IntVar(&port, "port", 8191, "HTTP服务端口")
	flag.StringVar(&handler.Token, "token", "", "访问令牌，不配置跳过验证")
	flag.StringVar(&config, "c", "xray.json", "配置文件, 默认(xray.json)")
	flag.IntVar(&offset, "offset", 0, "配置文件偏移量")
	flag.BoolVar(&handler.Serve.Reset, "reset", false, "是否重置配置文件")
	flag.BoolVar(&handler.Serve.Print, "print", false, "是否打印配置文件")
	flag.BoolVar(&ver, "version", false, "打印版本信息")
	flag.Parse()

	// ------------------------------------------------------------------------
	if ver {
		fmt.Printf("Xray v%s (https://github.com/XTLS/Xray-core)\nXweb %s (https://github.com/suisrc/xray-web)\n", core.Version(), Version())
		return
	}
	// ------------------------------------------------------------------------
	handler.Serve.Xrayc = fmt.Sprintf("%s.%d", config, offset) // 配置文件，尽量不动原始配置
	fmt.Printf("正在启动Xray,配置文件: %s -> %s\n", config, handler.Serve.Xrayc)
	go handler.Serve.StartXray() // 启动Xray
	// ------------------------------------------------------------------------
	fmt.Printf("HTTP服务启动,监听地址: %s:%d\n", addr, port)
	// http.ListenAndServe(fmt.Sprintf("%s:%d", addr, port), handler) // 启动HTTP服务
	// ------------------------------------------------------------------------
	// 启动HTTP服务， 并可优雅的终止
	srv := &http.Server{Addr: fmt.Sprintf("%s:%d", addr, port), Handler: handler}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sc
	log.Println("shutdown server ...")
	// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server shutdown:", err)
	}
	log.Println("server exiting")
}
