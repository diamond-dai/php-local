<?php

declare(strict_types=1);

$server = getenv('DB_HOST') ?: 'db';
$username = getenv('DB_USER') ?: 'app';
$database = getenv('DB_NAME') ?: 'app';

$_GET['server'] ??= $server;
$_GET['username'] ??= $username;
$_GET['db'] ??= $database;

function adminer_object(): \Adminer\Adminer
{
    $driver = 'server';
    $server = getenv('DB_HOST') ?: 'db';
    $username = getenv('DB_USER') ?: 'app';
    $password = getenv('DB_PASSWORD') ?: '';
    $database = getenv('DB_NAME') ?: 'app';

    if (!isset($_SESSION['pwds'][$driver][$server][$username])) {
        $_POST['auth'] = [
            'driver' => $driver,
            'server' => $server,
            'username' => $username,
            'password' => $password,
            'db' => $database,
        ];
    }

    return new class extends \Adminer\Adminer {
        public function credentials(): array
        {
            return [
                getenv('DB_HOST') ?: 'db',
                getenv('DB_USER') ?: 'app',
                getenv('DB_PASSWORD') ?: '',
            ];
        }

        public function database(): ?string
        {
            return getenv('DB_NAME') ?: 'app';
        }

        public function tablesPrint(array $tables): void
        {
            echo '<ul id="tables">' . \Adminer\script(
                "mixin(qs('#tables'), {onmouseover: menuOver, onmouseout: menuOut});"
            );
            foreach ($tables as $table => $status) {
                $table = (string) $table;
                $name = $this->tableName($status);
                if ($name === '' || $status['partition']) {
                    continue;
                }

                echo '<li><a href="' . \Adminer\h(\Adminer\ME) . 'table=' . urlencode($table) . '"'
                    . \Adminer\bold(
                        in_array($table, [
                            $_GET['table'],
                            $_GET['create'],
                            $_GET['indexes'],
                            $_GET['foreign'],
                            $_GET['trigger'],
                            $_GET['check'],
                            $_GET['view'],
                        ], true),
                        \Adminer\is_view($status) ? 'view' : 'structure'
                    )
                    . ' title="' . \Adminer\lang('Show structure') . '">'
                    . \Adminer\lang('structure') . '</a> ';

                echo '<a href="' . \Adminer\h(\Adminer\ME) . 'select=' . urlencode($table) . '"'
                    . \Adminer\bold(
                        $_GET['select'] === $table || $_GET['edit'] === $table,
                        'select'
                    )
                    . ' title="' . \Adminer\lang('Select data') . '">' . $name . '</a>';
            }
            echo "</ul>\n";
        }
    };
}

require __DIR__ . '/adminer.php';
