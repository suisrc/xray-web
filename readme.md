# xray for web

由于 xray 的 grpc 实现有点让你抓狂，而且对于配置， xray 有 conf.Config 和 core.Config 两个实现，grpc 使用的是 core.Config, 用起来很麻烦，所以这里提供了 xray for web，基于 conf.Config 配置，使用 http 的接口进行配置。  
  
没有使用 gin, iris, beego, echo, fasthttp 等框架，而是使用默认的 net/http， 这里只是简单的使用， 不需要复杂的配置。  
服务只支持 POST 请求， 服务中的 path 随意，不做限制，这里为了更好的穿透路由，服务使用 query 的 action 参数进行控制。  

## xray

https://github.com/XTLS/Xray-core  

## 调试

```sh

curl -s "http://127.0.0.1:8199/?action=healthz" | jq
# 测试, 不输出日志 
curl -sX POST "http://127.0.0.1:8199/?action=healthz" -d '{}' | jq

# body 内容来自文件 body.json
curl -sX POST "http://127.0.0.1:8199/?action=11" -d @body.json

# 初始化
make init
# 安装依赖
make tidy
# 编译
make build
# 编译 window 版本
make build-win
```