#!/bin/bash
set -euo pipefail

IMAGE="${1:-ghcr.io/diamond-dai/php-local:latest}"
PASS=0
FAIL=0
ERRORS=""

green() { printf "\033[32m%s\033[0m\n" "$1"; }
red()   { printf "\033[31m%s\033[0m\n" "$1"; }

run_test() {
  local name="$1"
  local cmd="$2"
  local expect="${3:-}"

  result=$(docker run --rm --entrypoint="" "$IMAGE" bash -c "$cmd" 2>&1) || true

  if [ -n "$expect" ]; then
    if echo "$result" | grep -qE "$expect"; then
      green "  PASS: $name"
      PASS=$((PASS + 1))
    else
      red "  FAIL: $name"
      red "    expected: $expect"
      red "    got: $result"
      FAIL=$((FAIL + 1))
      ERRORS="${ERRORS}\n  - ${name}"
    fi
  else
    # expect empty = just check exit code (command already ran)
    if docker run --rm --entrypoint="" "$IMAGE" bash -c "$cmd" >/dev/null 2>&1; then
      green "  PASS: $name"
      PASS=$((PASS + 1))
    else
      red "  FAIL: $name"
      red "    command failed: $cmd"
      FAIL=$((FAIL + 1))
      ERRORS="${ERRORS}\n  - ${name}"
    fi
  fi
}

echo "=========================================="
echo " Smoke Test: $IMAGE"
echo "=========================================="
echo ""

echo "[PHP]"
run_test "PHP 8.3 installed" "php -v" "PHP 8\.3"
run_test "ext: mysqli"       "php -m" "mysqli"
run_test "ext: pdo_mysql"    "php -m" "pdo_mysql"
run_test "ext: gd"           "php -m" "gd"
run_test "ext: zip"          "php -m" "zip"
run_test "ext: xml"          "php -m" "xml"
run_test "ext: mbstring"     "php -m" "mbstring"
run_test "ext: intl"         "php -m" "intl"
run_test "ext: curl"         "php -m" "curl"
run_test "ext: imagick"      "php -m" "imagick"
run_test "ext: exif"         "php -m" "exif"
run_test "ext: bcmath"       "php -m" "bcmath"
run_test "ext: opcache"      "php -m" "OPcache"
echo ""

echo "[Tools]"
run_test "composer"   "composer --version"  "Composer"
run_test "maildump"    "test -x /usr/local/bin/maildump && echo maildump" "maildump"
run_test "maildump saves mail" "rm -rf /tmp/maildump && printf 'From: sender@example.test\nTo: receiver@example.test\nSubject: smoke test\nContent-Type: text/plain; charset=UTF-8\n\nhello\n' | MAILDUMP_DIR=/tmp/maildump /usr/local/bin/maildump && test -f /tmp/maildump/*/*/*/raw.eml" ""
run_test "git"        "git --version"      "git version"
run_test "rsync"      "rsync --version"    "rsync"
run_test "mysql"      "mysql --version"    "mysql|mariadb"
run_test "Adminer bundled" "test -f /usr/local/share/adminer/adminer.php && php -l /usr/local/share/adminer/adminer.php" "No syntax errors"
run_test "Adminer autologin wrapper" "php -l /usr/local/share/adminer/index.php" "No syntax errors"
echo ""

echo "[Apache]"
run_test "apache2 installed"  "apache2 -v"                         "Apache"
run_test "mod_rewrite"        "apache2ctl -M 2>/dev/null"          "rewrite_module"
run_test "mod_include (SSI)"  "apache2ctl -M 2>/dev/null"          "include_module"
run_test "mod_headers"        "apache2ctl -M 2>/dev/null"          "headers_module"
run_test "mod_php"            "apache2ctl -M 2>/dev/null"          "php"
run_test "Adminer Alias"      "grep -F 'Alias /dbadmin/' /etc/apache2/sites-available/000-default.conf" "Alias /dbadmin/"
run_test "EnableSendfile off" "grep -c 'EnableSendfile off' /etc/apache2/apache2.conf" "1"
echo ""

echo "[PHP ini パス互換]"
run_test "旧パスに実体がある"          "ls /usr/local/etc/php/conf.d/99-upload.ini"    "99-upload.ini"
run_test "新パスからシンボリックリンク" "readlink /etc/php/8.3/apache2/conf.d"          "/usr/local/etc/php/conf.d"
run_test "PHPが ini を読める"          "php -i | grep upload_max_filesize"              "64M"
echo ""

echo "[Entrypoint]"
run_test "entrypoint exists"     "ls -la /usr/local/bin/entrypoint.sh"     "entrypoint.sh"
run_test "entrypoint executable" "test -x /usr/local/bin/entrypoint.sh && echo ok" "ok"
echo ""

echo "[Permissions]"
run_test "www-data user exists"   "id www-data"                              "www-data"
run_test "composer dir"           "stat -c '%U' /var/www/.composer"          "www-data"
run_test "maildump dir"           "stat -c '%U' /var/log/maildump"           "www-data"
run_test "sudo keeps PATH"        "grep 'env_keep' /etc/sudoers"             "PATH"
echo ""

echo "=========================================="
if [ $FAIL -eq 0 ]; then
  green "  ALL PASSED: ${PASS}/${PASS} tests"
else
  red "  FAILED: ${FAIL} / $((PASS + FAIL)) tests"
  red "  Failures:${ERRORS}"
fi
echo "=========================================="

exit $FAIL
