#!/bin/bash
# DBの内容を seed/sql/seed.sql にダンプする（次回 docker compose 初期化時に自動インポートされる）。
docker compose exec db /opt/mysql_db_dump.sh
