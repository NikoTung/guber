# guber
Guber is a tool to write application-IP mapping from nacos into local hosts.

## Why
In kubernate, your application's ip changed very time new deployment push and you want to access your application directly, bypass the gateway authentication.

## configuration

```
log:
  level: debug
service:
  - names:
      - "app-service"
      - "app-data"
    env: dev
    nacos:
      addr: http://127.0.0.1:8848
      username: nacos
      password: nacos
  - names:
      - "backen-service"
    env: test
    nacos:
      addr: http://10.0.0.1:8848
      username: nacos
      password: nacos

```

It will append some address-ip into hosts file

     app-service.dev 192.168.8.19
     app-data.dev 192.168.8.20
     backend-service.test 10.0.19.1