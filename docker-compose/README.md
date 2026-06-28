# PHP Local

素のPHP（PHP 8.3 + Apache）+ MariaDB のローカル開発環境。
Adminer と maildump を httpd コンテナに同梱している。

## 起動

公開イメージ（`ghcr.io/diamond-dai/php-local`）を使う場合:

```bash
docker compose up -d
docker compose logs -f
```

`.env` はコミット済み（非secretのデフォルト値）。案件・端末固有の上書きは
`.env.local`（gitignore対象）に書く。

ローカルビルドで試す場合は、`compose.override.yml.example` を
`compose.override.yml` にリネームしてから起動する:

```bash
cp compose.override.yml.example compose.override.yml
docker compose up -d --build
```

### MySQLあり / なし

| ファイル                    | 構成                          |
| --------------------------- | ----------------------------- |
| `compose.yml`        | httpd + MariaDB（既定）       |
| `compose.no-db.yml`  | httpd のみ（DBを使わない案件）|

DBを使わない案件では `compose.no-db.yml` を `compose.yml` に
リネームすれば、`task` コマンドがそのまま使える（元のファイルは退避・削除する）。

- アプリ: `http://localhost/`
- Adminer: `http://localhost/dbadmin/`

`HTTPD_PORT` を変更した場合は、どちらにも同じポートを付ける。

```text
HTTPD_PORT=8080
アプリ:    http://localhost:8080/
Adminer:   http://localhost:8080/dbadmin/
```

## ディレクトリ

| パス             | 用途                                              |
| ---------------- | ------------------------------------------------- |
| `htdocs/`        | DocumentRoot（`/var/www/html`）。ここにPHPを置く  |
| `seed/sql/`      | `seed.sql` を置くと初回起動時に自動インポートされる |
| `logs/`          | maildump が保存した送信メール                     |
| `mysql/my.cnf`   | MariaDB 設定                                      |

## Adminer

Adminer は httpd コンテナに同梱されており、追加コンテナは起動しない。
`/dbadmin/` を開くと `.env` の以下の値で自動ログインする。

```text
DB_HOST / DB_USER / DB_PASSWORD / DB_NAME
```

認証画面を省略しているため、httpd は既定で `127.0.0.1` にだけ公開する。

## DBへの接続（PHPアプリ）

`.env` の `DB_*` を使う。コンテナ内からは `DB_HOST=db` で接続する。

```php
$pdo = new PDO(
    "mysql:host=" . getenv('DB_HOST') . ";dbname=" . getenv('DB_NAME') . ";charset=utf8mb4",
    getenv('DB_USER'),
    getenv('DB_PASSWORD')
);
```

## ログ

Apache のアクセスログ・エラーログは stdout / stderr に転送しているので、
`docker compose logs -f` でそのまま追える。

## メール

PHP から送信したメールは外部へ送信せず、`./logs/` 配下へ保存する。
メールごとのディレクトリに次が作成される。

```text
raw.eml
meta.yaml
body.txt
body.html
attachments/
```

## シードのダンプ

現在のDB内容を `seed/sql/seed.sql` に書き出す（次回の初期化時に自動インポートされる）。

```bash
bash dump_seed.sh   # または: task dump-seed
```

## プロジェクト名

既定の Compose プロジェクト名は `php-local`。
複数案件を同時起動する場合は、`.env` の `COMPOSE_PROJECT_NAME` と `HTTPD_PORT` を案件ごとに変更する。
