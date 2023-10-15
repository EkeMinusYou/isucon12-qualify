# 最初にやること

# Shell環境を各サーバーにセットアップ

サーバーでShellとNeovim環境をセットアップ。全てのサーバーで実行する

```bash
make setup-shell SETUP_HOST=isucon-1
ssh isucon-1 "sudo -i -u isucon"
sudo passwd isucon
./setup.sh
```

Nginx/MySQL/Webappをローカルにコピーして、Git管理にする

※ Makefileの以下をアプリ名に書換えてから、makeを実行する

```Makefile
APP_NAME:=isuports
```

```bash
make setup-nginx
make setup-mysql
make setup-webapp
make setup-sysctl
```

## SSHの設定

`~/.ssh/config` のサンプル

```bash
Host isucon-1
  Hostname 13.231.219.177
  IdentityFile ~/.ssh/isucon-practice.pem
  User ubuntu
  RequestTTY yes

Host isucon-2
  Hostname 18.179.30.73
  IdentityFile ~/.ssh/isucon-practice.pem
  User ubuntu
  RequestTTY yes

Host isucon-3
  Hostname 54.178.138.79
  IdentityFile ~/.ssh/isucon-practice.pem
  User ubuntu
  RequestTTY yes
```

sshしつつユーザー切り替え

```bash
ssh isucon-1 "sudo -i -u isucon"
```

## Docker剥がし

`etc/systemd/system/$(サービス名) `に以下のように追記する

```bash
WorkingDirectory=/home/isucon/webapp/go
ExecStart=/home/isucon/webapp/go/isuports
```

また、元々はDockerで定義されていた環境変数は、`etc/sytemd/system/${サービス名}` で以下のように定義する

```bash
Environment=ISUCON_DB_HOST=192.168.0.12
Environment=ISUCON_DB_PORT=3306
Environment=ISUCON_DB_USER=isucon
Environment=ISUCON_DB_PASSWORD=isucon
Environment=ISUCON_DB_NAME=isuports
```

webapp/go配下のビルドするMakefileのサンプル

```Makefile
isuports:
	go build -o isuports ./...
```

## ログと計測の準備

最終的に元に戻すこと

### Nginx

`nginx/nginx.conf` でログ出力を以下のように書き換える

```
  log_format json escape=json '{"time":"$time_local",'
                              '"host":"$remote_addr",'
                              '"forwardedfor":"$http_x_forwarded_for",'
                              '"req":"$request",'
                              '"status":"$status",'
                              '"method":"$request_method",'
                              '"uri":"$request_uri",'
                              '"body_bytes":$body_bytes_sent,'
                              '"referer":"$http_referer",'
                              '"ua":"$http_user_agent",'
                              '"request_time":$request_time,'
                              '"cache":"$upstream_http_x_cache",'
                              '"runtime":"$upstream_http_x_runtime",'
                              '"response_time":"$upstream_response_time",'
                              '"vhost":"$host"}';
```

```
access_log /var/log/nginx/access.log json;
```

### Goのprofile

以下のようにProfileの設定をする。

```go
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
			<-sig
			pprof.StopCPUProfile()
			f.Close()
			os.Exit(0)
		}()
	}

	Run()
}
```

systemdでファイル名を指定する

```
ExecStart=/home/isucon/webapp/go/isuports -cpuprofile cpu.pprof
```

pdf出力のためgarphvizをinstall

```bash
brew install graphviz
```

# 必要に応じてやること

## Nginxの向き先を変える

`nginx/sites-available/${サービス名}` の以下を書き変える。proxy_passで `127.0.0.1` となっている箇所を、対象のプライベートIPに変更する。


```conf
  location / {
    try_files $uri /index.html;
  }

  location ~ ^/(api|initialize) {
    proxy_set_header Host $host;
    proxy_read_timeout 600;
    proxy_pass http://127.0.0.1:3000;
  }

  location /auth/ {
    proxy_set_header Host $host;
    proxy_pass http://127.0.0.1:3001;
  }
```

## 別サーバーのDBにアクセスする

対象のサーバーの`mysql/mysql.conf.d/mysqld.confのbind-address`を無効にする

```
# bind-address		= 127.0.0.1
```

アプリのsystemdの設定で環境変数を対象のサーバーのIPアドレスに置換える

```
Environment=ISUCON_DB_HOST=172.31.32.96
Environment=ISUCON_DB_PORT=3306
Environment=ISUCON_DB_USER=isucon
Environment=ISUCON_DB_PASSWORD=isucon
Environment=ISUCON_DB_NAME=isucon
```

MySQLのisuconユーザーがlocalhost以外からの接続を受け入れているか確認する。%になっていたらOK

```sql
mysql -u isucon -p
mysql> SELECT user, host FROM mysql.user;
+------------------+-----------+
| user             | host      |
+------------------+-----------+
| isucon           | %         |
| debian-sys-maint | localhost |
| mysql.infoschema | localhost |
| mysql.session    | localhost |
| mysql.sys        | localhost |
| root             | localhost |
+------------------+-----------+
6 rows in set (0.00 sec)
```

なっていなかったら、以下のコマンドでユーザーと権限を追加する

```sql
CREATE USER "isucon"@"%" IDENTIFIED BY "isucon";
GRANT ALL PRIVILEGES ON *.* TO "isucon"@"%";
```


## pgoを有効にする

以下のようにmake targetを書き換える

```makefile
isuports:
	cp cpu.pprof default.pgo || true
	go build -o isuports ./... -pgo=auto
```

## ER図を出力

サーバーで以下を実行しtblsをインストールし、ドキュメントを出力する。isuportsの部分は対象のDB名なのでアプリの名称によって、適宜変えること。

```bash
brew install k1LoW/tap/tbls
source ~/.zshrc
tbls doc mysql://isucon:isucon@localhost:3306/isuports ./dbdoc
```

その後、ローカルで以下を実行して、ローカルにファイルをコピーして、git管理にする

```bash
mkdir -p dbdoc
rsync -az -e ssh ubuntu@isucon-1:/home/isucon/dbdoc/ dbdoc/ --rsync-path="sudo rsync"
```

# 秘伝のタレ

## nginx.conf

worker_rlimit_nofileはworker_connectionsの4倍程度で設定する

```nginx
worker_rlimit_nofile  4096;
events {
  worker_connections 1024;
}
```

alp用のログ設定

```nginx
http {
  log_format json escape=json '{"time":"$time_local",'
                              '"host":"$remote_addr",'
                              '"forwardedfor":"$http_x_forwarded_for",'
                              '"req":"$request",'
                              '"status":"$status",'
                              '"method":"$request_method",'
                              '"uri":"$request_uri",'
                              '"body_bytes":$body_bytes_sent,'
                              '"referer":"$http_referer",'
                              '"ua":"$http_user_agent",'
                              '"request_time":$request_time,'
                              '"cache":"$upstream_http_x_cache",'
                              '"runtime":"$upstream_http_x_runtime",'
                              '"response_time":"$upstream_response_time",'
                              '"vhost":"$host"}';

  access_log /var/log/nginx/access.log json;
  error_log /var/log/nginx/error.log;
}
```

基本設定

```nginx
http {
  sendfile    on;
  tcp_nopush  on;
  tcp_nodelay on;
  types_hash_max_size 2048;
  server_tokens    off;
}
```

nginxとupstreamのkeepalive設定。app側も対応必要

```nginx
http {
  upstream app {
    server 192.100.0.1:5000;
    keepalive 60;
  }

  location /api/ {
    proxy_set_header Host $host;
    proxy_read_timeout 600;
    proxy_pass http://app;

    # この二つの設定はkeepaliveに必須
    proxy_http_version 1.1;
    proxy_set_header Connection "";
  }
```

## mysqld.cnf

スロークエリを有効にする。最後に無効にすること。

```
slow_query_log		= 1
slow_query_log_file	= /var/log/mysql/mysql-slow.log
long_query_time = 0
```

ディスクイメージをメモリー上にバッファする

```
innodb_buffer_pool_size = 1GB
innodb_flush_log_at_trx_commit = 2
innodb_flush_method = O_DIRECT
```

isuconでクラスタ構成を使わない場合disable-log-binを1にする

```
disable-log-bin = 1
```

## MySQLのLimitnofileを増やす


`etc/systemd/system/mysql.service.d/limits.conf` で以下のように書いて、`make deploy-mysql`

```bash
[Service]
LimitNOFILE=1006500
```
## カーネルパラメーター

`etc/systemd/sysctl.conf` で以下を書いて、`make deploy-sysctl` で全サーバーに適用

```
net.core.somaxconn = 8192
net.ipv4.ip_local_port_range = 10000    60999
```
