# certbot-go

一个支持多域名自动申请、续期SSL证书的工具

## 使用方式

1. 前往 [Release](https://github.com/AyakuraYuki/certbot-go/releases) 下载最新版本的可执行文件
2. 准备好阿里云 Access Key，至少需要分配 `AliyunDNSFullAccess` 权限
3. 准备好下面的配置文件，修改成你的真实配置

    ```yaml
    # certbot-go 配置文件
    # ==================
    
    # Let's Encrypt 账号邮箱，用于证书到期提醒
    email:
    
    # ACME 目录地址
    # 生产环境: https://acme-v02.api.letsencrypt.org/directory
    # 测试环境: https://acme-staging-v02.api.letsencrypt.org/directory
    # ⚠️ 首次调试建议先用 staging，避免触发 Let's Encrypt 的速率限制
    acme_directory: https://acme-staging-v02.api.letsencrypt.org/directory
    
    # 证书存储目录，每个证书会创建一个子目录
    # 子目录内包含: fullchain.pem, privkey.pem, chain.pem, metadata.json
    cert_dir: /etc/letsencrypt/live
    
    # ACME 账号信息存储目录
    account_dir: /etc/letsencrypt/accounts
    
    # 续签检查间隔（daemon 模式下有效）
    # Let's Encrypt 证书 90 天过期，certbot-go 在到期前 renew_before 时间内才会真正续签
    check_interval: 12h
    
    # 在证书到期前多久开始续签
    renew_before: 720h  # 30 天
    
    # 阿里云 DNS API 凭证
    # 用于在阿里云云解析上自动创建/删除 ACME challenge TXT 记录
    # 建议使用 RAM 子账号，最小权限: AliyunDNSFullAccess
    # 支持环境变量引用: $ALICLOUD_ACCESS_KEY, $ALICLOUD_SECRET_KEY
    alidns:
      access_key_id: "$ALICLOUD_ACCESS_KEY"
      access_key_secret: "$ALICLOUD_SECRET_KEY"
      # region_id: cn-hangzhou  # 目前 alidns API 不区分 region，保留字段
    
    # 证书列表
    # ==========
    #
    # 支持两种模式:
    #
    # 模式 1 - CNAME 委托 (天翼云等不支持 API 的 DNS 服务商):
    #   设置 challenge_delegate，程序通过阿里云 API 操作委托域名上的 TXT 记录
    #   天翼云 DNS 需要一次性手动添加 CNAME:
    #     _acme-challenge.example.com CNAME _acme-challenge.example.proxy-acme.com
    #
    # 模式 2 - 直接模式 (域名本身就在阿里云 DNS 解析):
    #   不设置 challenge_delegate，程序直接在阿里云 DNS 上操作 _acme-challenge TXT 记录
    #
    certificates:
      # 示例 1: CNAME 委托模式 — 通配符证书 + 裸域名
      # 在主域名 DNS 手动添加:
      #   _acme-challenge.example.com  CNAME  _acme-challenge.example.proxy-acme.com
      - name: example.com
        domains:
          - "example.com"
          - "*.example.com"
        challenge_delegate: "example.proxy-acme.com"
    
      # 示例 2: 直接模式 — 域名在阿里云 DNS 上，无需委托
      # 不需要任何手动操作，程序直接调 API 完成验证
      # - name: ali-hosted.com
      #   domains:
      #     - "ali-hosted.com"
      #     - "*.ali-hosted.com"
      #   # 注意: 不写 challenge_delegate 即为直接模式
    
      # 示例 3: 直接模式 — 多个独立二级域名证书
      # - name: api-certs
      #   domains:
      #     - "api.ali-hosted.com"
      #     - "admin.ali-hosted.com"
    
      # 示例 4: CNAME 委托 — 另一个一级域名
      # 在主域名 DNS 手动添加:
      #   _acme-challenge.another-domain.com  CNAME  _acme-challenge.another-domain.proxy-acme.com
      # - name: another-domain.com
      #   domains:
      #     - "another-domain.com"
      #     - "*.another-domain.com"
      #   challenge_delegate: "another-domain.proxy-acme.com"
    
    ```

4. 配置系统服务，或者单次使用

   如需配置系统服务，可以选择 `systemd` 或者 `supervisor`

    1. `systemd` 需准备如下的配置，注册到 systemd；注意，你可能需要修改部分路径信息，如果在 `config.yaml` 没有配置阿里云 Access Key 的，还需要在这里增加环境变量配置。
       ```unit file (systemd)
       [Unit]
       Description=certbot-go ACME certificate manager
       Wants=network-online.target
       After=network-online.target
       
       [Service]
       Type=simple
       ExecStart=/usr/local/bin/certbot-go --config /etc/certbot-go/config.yaml
       Restart=on-failure
       RestartSec=10
       
       # Security hardening
       NoNewPrivileges=yes
       PrivateTmp=yes
       ProtectSystem=strict
       ReadWritePaths=/etc/letsencrypt
       
       # Environment variables for Alibaba Cloud credentials
       # Option 1: inline (not recommended for production)
       # Environment=ALICLOUD_ACCESS_KEY=your-ak
       # Environment=ALICLOUD_SECRET_KEY=your-sk
       
       # Option 2: environment file (recommended)
       # EnvironmentFile=-/etc/certbot-go/.env
       
       [Install]
       WantedBy=multi-user.target
       ```

    2. `supervisor` 需准备如下的配置，存放在 supervisor 服务配置目录；同样的，你可能需要修改部分路径信息，如果在 `config.yaml` 没有配置阿里云 Access Key 的，还需要在这里增加环境变量配置。
       ```text
       # supervisor service config
       [program:certbot-go]
       directory = /etc/certbot-go
       command = /usr/local/bin/certbot-go --config /etc/certbot-go/config.yaml
       stopasgroup = true
       killasgroup = true
       autostart = true
       startsecs = 5
       autorestart = true
       startretries = 10
       user = root
       redirect_stderr = false
       
       stdout_logfile = /var/log/certbot-go.log
       stdout_logfile_maxbytes = 32MB
       stdout_logfile_backups = 2
       
       stderr_logfile = /var/log/certbot-go.err.log
       stderr_logfile_maxbytes = 32MB
       stderr_logfile_backups = 2
       
       environment = ALICLOUD_ACCESS_KEY="",ALICLOUD_SECRET_KEY=""
       ```

   如果只想单次运行，可以使用命令：

    ```shell
    certbot-go --config config.yaml --once
    ```
