<?php

declare(strict_types=1);

// =============================================================================
// 動作確認用スターターページ。
// PHP/DB/メール送信の疎通を確認できる。実際の開発ではこのファイルを差し替える。
// =============================================================================

$dbHost = getenv('DB_HOST') ?: 'db';
$dbUser = getenv('DB_USER') ?: 'app';
$dbPass = getenv('DB_PASSWORD') ?: '';
$dbName = getenv('DB_NAME') ?: 'app';

// DB接続チェック
$dbStatus = 'unknown';
$dbError = '';
try {
    $pdo = new PDO(
        "mysql:host={$dbHost};dbname={$dbName};charset=utf8mb4",
        $dbUser,
        $dbPass,
        [PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION]
    );
    $version = $pdo->query('SELECT VERSION()')->fetchColumn();
    $dbStatus = "connected (MySQL/MariaDB {$version})";
} catch (Throwable $e) {
    $dbStatus = 'failed';
    $dbError = $e->getMessage();
}

// メール送信テスト（?sendmail=1 で実行。maildump が ./logs/ に保存する）
$mailResult = '';
if (isset($_GET['sendmail'])) {
    $ok = mail(
        'test@example.test',
        'maildump test ' . date('H:i:s'),
        "これはテストメールです。\n./logs/ 配下に保存されます。",
        'From: noreply@example.test'
    );
    $mailResult = $ok ? 'mail() を呼び出しました。./logs/ を確認してください。' : 'mail() が false を返しました。';
}

?>
<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>php-local</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 720px; margin: 40px auto; padding: 0 16px; line-height: 1.7; }
  h1 { border-bottom: 2px solid #333; padding-bottom: 8px; }
  table { border-collapse: collapse; width: 100%; margin: 16px 0; }
  th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
  th { background: #f5f5f5; width: 180px; }
  .ok { color: #0a0; font-weight: bold; }
  .ng { color: #c00; font-weight: bold; }
  code { background: #f0f0f0; padding: 2px 6px; border-radius: 3px; }
  a.button { display: inline-block; padding: 8px 16px; background: #333; color: #fff; text-decoration: none; border-radius: 4px; margin-right: 8px; }
</style>
</head>
<body>
<h1>php-local 動作確認</h1>

<table>
  <tr><th>PHP バージョン</th><td><?= htmlspecialchars(PHP_VERSION) ?></td></tr>
  <tr>
    <th>DB 接続</th>
    <td>
      <?php if ($dbError === ''): ?>
        <span class="ok"><?= htmlspecialchars($dbStatus) ?></span>
      <?php else: ?>
        <span class="ng"><?= htmlspecialchars($dbStatus) ?></span><br>
        <small><?= htmlspecialchars($dbError) ?></small>
      <?php endif; ?>
    </td>
  </tr>
  <tr><th>DocumentRoot</th><td><code>./docker-compose/htdocs/</code></td></tr>
</table>

<h2>リンク</h2>
<p>
  <a class="button" href="/dbadmin/">Adminer（DB管理）</a>
  <a class="button" href="?sendmail=1">メール送信テスト</a>
</p>
<?php if ($mailResult !== ''): ?>
  <p><strong><?= htmlspecialchars($mailResult) ?></strong></p>
<?php endif; ?>

<h2>このページについて</h2>
<p>
  これは動作確認用のスターターページです。開発を始めるときは
  <code>docker-compose/htdocs/</code> 配下を自由に差し替えてください。
</p>
</body>
</html>
