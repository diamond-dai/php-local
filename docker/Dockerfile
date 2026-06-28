# =============================================================================
# Stage 1: Build Go binary (maildump)
# =============================================================================
FROM golang:1.23-bookworm AS go-builder

WORKDIR /build
COPY go/go.mod go/go.sum ./
RUN go mod download
COPY go/ ./
# maildump: sendmail互換のメールダンプツール（本文・添付・メタをファイル保存）
RUN CGO_ENABLED=0 go build -ldflags='-extldflags -static' -o /maildump ./cmd/maildump

# =============================================================================
# Stage 2: Download tools (composer + Adminer)
# =============================================================================
FROM debian:bookworm-slim AS downloader

# 固定バージョン（最新化する場合はここを上げて再ビルド。--build-arg で上書きも可）
ARG COMPOSER_VERSION=2.10.1
ARG ADMINER_VERSION=5.4.2

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# composer（phar直取得。このステージにPHPは不要）
RUN curl -o /usr/local/bin/composer -fSL \
    "https://getcomposer.org/download/${COMPOSER_VERSION}/composer.phar" && \
    chmod +x /usr/local/bin/composer

# Adminer（MySQL/MariaDB版）
RUN mkdir -p /usr/local/share/adminer && \
    curl -o /usr/local/share/adminer/adminer.php -fSL \
    "https://www.adminer.org/static/download/${ADMINER_VERSION}/adminer-${ADMINER_VERSION}-mysql.php"

# =============================================================================
# Stage 3: Final image (debian:bookworm-slim + PHP 8.3 + Apache)
# =============================================================================
FROM debian:bookworm-slim

# sury.org リポジトリを追加（PHP 8.3）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl gnupg2 lsb-release && \
    curl -fsSL https://packages.sury.org/php/apt.gpg | gpg --dearmor -o /usr/share/keyrings/sury-php.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ $(lsb_release -sc) main" \
      > /etc/apt/sources.list.d/sury-php.list && \
    rm -rf /var/lib/apt/lists/*

# Apache + PHP 8.3 + システムパッケージ
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Apache
    apache2 \
    libapache2-mod-php8.3 \
    # PHP本体 + 拡張
    php8.3 \
    php8.3-mysql \
    php8.3-gd \
    php8.3-zip \
    php8.3-xml \
    php8.3-mbstring \
    php8.3-intl \
    php8.3-curl \
    php8.3-imagick \
    php8.3-exif \
    php8.3-bcmath \
    php8.3-opcache \
    # システムツール
    default-mysql-client \
    git \
    vim \
    less \
    sudo \
    zip \
    unzip \
    rsync \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Apache のアクセス/エラーログを stdout/stderr に転送
# （docker compose logs -f でログが見えるようにする。公式 php:apache・httpd と同じ方式）
RUN ln -sf /dev/stdout /var/log/apache2/access.log \
    && ln -sf /dev/stderr /var/log/apache2/error.log \
    && ln -sf /dev/stdout /var/log/apache2/other_vhosts_access.log

# 旧公式PHPイメージとの互換パス
# conf.d の実体を /usr/local/etc/php/conf.d/ (旧パス) に統一し、新パスからシンボリックリンク
RUN mkdir -p /usr/local/etc/php/conf.d && \
    cp /etc/php/8.3/apache2/conf.d/* /usr/local/etc/php/conf.d/ 2>/dev/null || true && \
    rm -rf /etc/php/8.3/apache2/conf.d && \
    ln -s /usr/local/etc/php/conf.d /etc/php/8.3/apache2/conf.d && \
    rm -rf /etc/php/8.3/cli/conf.d && \
    ln -s /usr/local/etc/php/conf.d /etc/php/8.3/cli/conf.d

# ビルド成果物・ツールをコピー
COPY --from=go-builder /maildump /usr/local/bin/maildump
COPY --from=downloader /usr/local/bin/composer /usr/local/bin/composer
COPY --from=downloader /usr/local/share/adminer /usr/local/share/adminer
RUN chmod +x /usr/local/bin/maildump /usr/local/bin/composer

# Apacheモジュール（rewrite + SSI/mod_include + headers）
RUN a2enmod rewrite include headers

# mod_php は prefork MPM が必要
RUN a2dismod mpm_event 2>/dev/null || true && \
    a2enmod mpm_prefork 2>/dev/null || true

# Docker volume マウント時にキャッシュが消えない問題対策
RUN echo 'EnableSendfile off' >> /etc/apache2/apache2.conf

# 起動時の AH00558 警告を抑制
RUN echo 'ServerName localhost' >> /etc/apache2/apache2.conf

# composer 用ディレクトリ
RUN mkdir -p /var/www/.composer && \
    chown -R www-data:www-data /var/www/.composer

# sudo 経由で PATH を維持
RUN sed -i -e 's/^Defaults.*env_reset/Defaults env_keep += "PATH"/g' /etc/sudoers

# 設定ファイル
COPY conf/php-upload.ini    /usr/local/etc/php/conf.d/99-upload.ini
COPY conf/php-debug.ini     /usr/local/etc/php/conf.d/99-debug.ini
COPY conf/sendmail.ini      /usr/local/etc/php/conf.d/99-sendmail.ini
COPY conf/apache-vhost.conf /etc/apache2/sites-available/000-default.conf
COPY conf/adminer-index.php /usr/local/share/adminer/index.php

# entrypoint
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# maildump の保存先
RUN mkdir -p /var/log/maildump && chown www-data:www-data /var/log/maildump

# デフォルト環境変数
# ※ パスワード系(DB_PASSWORD/MYSQL_ROOT_PASSWORD)は機密のため ENV デフォルトを置かない。
#   docker history でイメージに焼き込まれるのを防ぐため、compose の .env で必ず供給すること。
ENV TZ=Asia/Tokyo \
    DB_HOST=db \
    DB_USER=app \
    DB_NAME=app \
    APACHE_RUN_USER=www-data \
    APACHE_RUN_GROUP=www-data \
    APACHE_LOG_DIR=/var/log/apache2

WORKDIR /var/www/html

EXPOSE 80

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["apache2ctl", "-D", "FOREGROUND"]
