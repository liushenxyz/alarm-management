# alert-management

### 访问地址

http://localhost:8080/swagger/index.html#/

### 配置文件

```yaml
# /app/configs/config.yaml
server:
  addr: 0.0.0.0
  port: 8080
zabbix:
  url: http://127.0.0.1/api_jsonrpc.php
  token: 532f89f33e21e96509a3a05619163a33262ec073db94bc2c9aa9da1086bf381e
elasticsearch:
  url: https://127.0.0.1:9200
  username: elastic
  password: elastic
basic:
  username: admin
  password: admin
```

### 运行

```bash
docker build --tag alert-management .
docker-compose up -d
```