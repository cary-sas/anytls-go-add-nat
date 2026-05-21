# AnyTLS

AnyTLS 是一个用于缓解 TLS in TLS 指纹问题的代理协议实现。本仓库的客户端支持普通 SOCKS5/HTTP 入站，也支持 Linux 路由器上的 TCP 透明代理入站。

- SOCKS5/HTTP 应用代理：适合浏览器、curl、下载工具等手动配置代理。
- NAT 透明代理：适合配合路由器 iptables REDIRECT 使用，让局域网设备无需逐台配置。
- 连接复用：客户端和服务端之间复用 anytls 会话，降低代理延迟。

[用户常见问题](./docs/faq.md)

[协议文档](./docs/protocol.md)

[URI 格式](./docs/uri_scheme.md)

## 快速使用

示例程序默认使用 `InsecureSkipVerify`，适合测试和自建环境；生产使用时请理解对应的 TLS 信任风险。

### 服务端

```bash
./anytls-server -l 0.0.0.0:8443 -p 'your_password'
```

### 客户端：本机 SOCKS5/HTTP 代理

```bash
./anytls-client -s server_ip:8443 -p 'your_password' -socks 127.0.0.1:1080
```

兼容旧参数：

```bash
./anytls-client -s server_ip:8443 -p 'your_password' -l 127.0.0.1:1080
```

URI 写法：

```bash
./anytls-client -s 'anytls://your_password@server_ip:8443' -socks 127.0.0.1:1080
```

### 客户端：路由器透明代理

透明代理只支持 Linux，因为客户端需要通过 `SO_ORIGINAL_DST` 读取 iptables REDIRECT 前的真实目标地址。当前 NAT 模式只透明转发 TCP；UDP 和普通 DNS 不会被自动代理。需要 DNS 代理时，建议在路由器上另配 DoH/DoT、dnsmasq 上游，或让客户端使用支持远端解析的 SOCKS/HTTP 代理。

```bash
./anytls-client \
  -s server_ip:8443 \
  -p 'your_password' \
  -socks 127.0.0.1:1080 \
  -nat 0.0.0.0:3333
```

路由器侧需要自行配置 iptables，将需要透明代理的 TCP 流量 REDIRECT 到 `-nat` 指定的监听端口；上面的 `3333` 只是示例端口。规则里还需要排除 SSH、局域网地址和 anytls 服务器地址，避免断连或转发循环。

测试 SOCKS5：

```bash
curl -x socks5h://127.0.0.1:1080 https://ifconfig.me
```

测试透明代理时，在已经被路由器 REDIRECT 的局域网设备上直接访问：

```bash
curl https://ifconfig.me
```

