<?php
// Seintech Panel SSO & Modern Theme Wrapper for Adminer
declare(strict_types=1);

chdir(__DIR__);
@ini_set('display_errors', '0');
@ini_set('log_errors', '1');

// 1. Process One-Time SSO Token from Seintech Panel
$token = $_GET['token'] ?? '';
if (is_string($token) && preg_match('/^[a-f0-9]{32,64}$/', $token)) {
    $tokenFile = '/var/lib/seintech/pma_tokens/' . $token . '.json';
    if (file_exists($tokenFile)) {
        $raw = @file_get_contents($tokenFile);
        $data = @json_decode($raw, true);
        @unlink($tokenFile);

        if ($data && time() <= ($data['expires'] ?? 0)) {
            $isPg = ($data['type'] ?? '') === 'postgres';
            $driver = $isPg ? 'pgsql' : 'server';
            $server = '127.0.0.1'; // 127.0.0.1 ensures TCP loopback authentication
            $username = (string)($data['user'] ?? '');
            $password = (string)($data['pass'] ?? '');
            $db = (string)($data['db'] ?? '');

            if (session_status() === PHP_SESSION_NONE) {
                session_name('adminer_sid');
                @session_start();
            }

            // Store credentials in session for Adminer credentials() hook
            $_SESSION['seintech_auth'] = [
                'driver'   => $driver,
                'server'   => $server,
                'username' => $username,
                'password' => $password,
                'db'       => $db,
                'time'     => time(),
            ];

            if (!isset($_SESSION['pwds'])) {
                $_SESSION['pwds'] = [];
            }
            $_SESSION['pwds'][$driver][$server][$username] = $password;

            session_write_close();

            $target = 'index.php?' . $driver . '=' . urlencode($server) . '&username=' . urlencode($username);
            if (!empty($db)) {
                $target .= '&db=' . urlencode($db);
            }
            header('Location: ' . $target);
            exit;
        }
    }
}

// 2. Set session name if session is not started yet
if (session_status() === PHP_SESSION_NONE) {
    session_name('adminer_sid');
}

// 3. Clear SSO credentials on explicit logout
if (isset($_GET['logout'])) {
    if (session_status() === PHP_SESSION_NONE) {
        session_name('adminer_sid');
        @session_start();
    }
    unset($_SESSION['seintech_auth']);
    unset($_SESSION['pwds']);
    session_write_close();
}

// 4. Custom Adminer object supporting both Adminer 4 and 5
function adminer_object() {
    if (class_exists('Adminer\\Adminer')) {
        if (!class_exists('SeintechAdminer5', false)) {
            class SeintechAdminer5 extends Adminer\Adminer {
                function name() {
                    return '<a href="/databases" class="seintech-brand" style="text-decoration:none;color:inherit;"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M4 7v10c0 2 3.5 3 8 3s8-1 8-3V7M4 7c0 2 3.5 3 8 3s8-1 8-3M4 7c0-2 3.5-3 8-3s8 1 8 3m0 5c0 2-3.5 3-8 3s-8-1-8-3"/></svg> Seintech Database Manager</a>';
                }
                function head() {
                    $v = file_exists(__DIR__ . '/adminer.css') ? filemtime(__DIR__ . '/adminer.css') : time();
                    echo '<link rel="stylesheet" type="text/css" href="/adminer/adminer.css?v=' . $v . '">';
                    echo '<style>
                    #menu { position: fixed !important; top: 0 !important; left: 0 !important; bottom: 0 !important; width: 260px !important; margin: 0 !important; padding: 20px 16px !important; z-index: 100 !important; background: #ffffff !important; border-right: 1px solid #e2e8f0 !important; box-shadow: 1px 0 3px rgba(0,0,0,0.03) !important; overflow-y: auto !important; }
                    #menu h1, h1 { background: transparent !important; border: none !important; border-bottom: 1px solid #e2e8f0 !important; padding: 0 0 14px 0 !important; margin: 0 0 16px 0 !important; font-size: 14.5px !important; font-weight: 700 !important; }
                    #menu .version { display: inline-block !important; font-size: 11px !important; color: #64748b !important; background: #f1f5f9 !important; padding: 1px 6px !important; border-radius: 4px !important; margin-left: 6px !important; }
                    #version { display: none !important; }
                    #breadcrumb { position: fixed !important; top: 0 !important; left: 260px !important; right: 0 !important; height: 52px !important; line-height: 52px !important; margin: 0 !important; padding: 0 32px !important; background: #ffffff !important; border-bottom: 1px solid #e2e8f0 !important; z-index: 90 !important; display: flex !important; align-items: center !important; font-size: 13px !important; color: #64748b !important; white-space: nowrap !important; overflow-x: auto !important; }
                    #breadcrumb a { color: #4f46e5 !important; font-weight: 600 !important; }
                    .logout, p.logout, form.logout { position: fixed !important; top: 10px !important; right: 24px !important; margin: 0 !important; padding: 0 !important; z-index: 95 !important; }
                    .logout input[type="submit"], .logout button { font-size: 12px !important; font-weight: 600 !important; padding: 6px 14px !important; color: #dc2626 !important; background: #fee2e2 !important; border: 1px solid #fecaca !important; border-radius: 6px !important; cursor: pointer !important; }
                    #content { margin-left: 260px !important; margin-top: 52px !important; margin-right: 0 !important; margin-bottom: 0 !important; padding: 28px 36px !important; max-width: 1400px !important; min-height: calc(100vh - 52px) !important; position: relative !important; }
                    h2 { font-size: 20px !important; font-weight: 700 !important; color: #0f172a !important; margin: 0 0 20px 0 !important; padding: 0 !important; background: transparent !important; border: none !important; display: flex !important; align-items: center !important; gap: 8px !important; letter-spacing: -0.02em !important; }
                    </style>';
                    return true;
                }
                function css() {
                    $v = file_exists(__DIR__ . '/adminer.css') ? filemtime(__DIR__ . '/adminer.css') : time();
                    return ['adminer.css?v=' . $v];
                }
                function credentials() {
                    $auth = $_SESSION['seintech_auth'] ?? [];
                    if (!empty($auth['password'])) {
                        return [
                            $auth['server'] ?? '127.0.0.1',
                            $auth['username'] ?? '',
                            $auth['password'] ?? ''
                        ];
                    }
                    return array(SERVER, $_GET["username"] ?? "", get_password());
                }
                function database() {
                    if (!empty($_GET['db'])) {
                        return (string)$_GET['db'];
                    }
                    $auth = $_SESSION['seintech_auth'] ?? [];
                    if (!empty($auth['db'])) {
                        return (string)$auth['db'];
                    }
                    return parent::database();
                }
                function login($login, $password) {
                    return true;
                }
            }
        }
        return new SeintechAdminer5();
    }

    if (!class_exists('SeintechAdminer4', false)) {
        class SeintechAdminer4 extends Adminer {
            function name() {
                return '<a href="/databases" class="seintech-brand" style="text-decoration:none;color:inherit;"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M4 7v10c0 2 3.5 3 8 3s8-1 8-3V7M4 7c0 2 3.5 3 8 3s8-1 8-3M4 7c0-2 3.5-3 8-3s8 1 8 3m0 5c0 2-3.5 3-8 3s-8-1-8-3"/></svg> Seintech Database Manager</a>';
            }
            function head() {
                $v = file_exists(__DIR__ . '/adminer.css') ? filemtime(__DIR__ . '/adminer.css') : time();
                echo '<link rel="stylesheet" type="text/css" href="/adminer/adminer.css?v=' . $v . '">';
                echo '<style>
                #menu { position: fixed !important; top: 0 !important; left: 0 !important; bottom: 0 !important; width: 260px !important; margin: 0 !important; padding: 20px 16px !important; z-index: 100 !important; background: #ffffff !important; border-right: 1px solid #e2e8f0 !important; box-shadow: 1px 0 3px rgba(0,0,0,0.03) !important; overflow-y: auto !important; }
                #menu h1, h1 { background: transparent !important; border: none !important; border-bottom: 1px solid #e2e8f0 !important; padding: 0 0 14px 0 !important; margin: 0 0 16px 0 !important; font-size: 14.5px !important; font-weight: 700 !important; }
                #menu .version { display: inline-block !important; font-size: 11px !important; color: #64748b !important; background: #f1f5f9 !important; padding: 1px 6px !important; border-radius: 4px !important; margin-left: 6px !important; }
                #version { display: none !important; }
                #breadcrumb { position: fixed !important; top: 0 !important; left: 260px !important; right: 0 !important; height: 52px !important; line-height: 52px !important; margin: 0 !important; padding: 0 32px !important; background: #ffffff !important; border-bottom: 1px solid #e2e8f0 !important; z-index: 90 !important; display: flex !important; align-items: center !important; font-size: 13px !important; color: #64748b !important; white-space: nowrap !important; overflow-x: auto !important; }
                #breadcrumb a { color: #4f46e5 !important; font-weight: 600 !important; }
                .logout, p.logout, form.logout { position: fixed !important; top: 10px !important; right: 24px !important; margin: 0 !important; padding: 0 !important; z-index: 95 !important; }
                .logout input[type="submit"], .logout button { font-size: 12px !important; font-weight: 600 !important; padding: 6px 14px !important; color: #dc2626 !important; background: #fee2e2 !important; border: 1px solid #fecaca !important; border-radius: 6px !important; cursor: pointer !important; }
                #content { margin-left: 260px !important; margin-top: 52px !important; margin-right: 0 !important; margin-bottom: 0 !important; padding: 28px 36px !important; max-width: 1400px !important; min-height: calc(100vh - 52px) !important; position: relative !important; }
                h2 { font-size: 20px !important; font-weight: 700 !important; color: #0f172a !important; margin: 0 0 20px 0 !important; padding: 0 !important; background: transparent !important; border: none !important; display: flex !important; align-items: center !important; gap: 8px !important; letter-spacing: -0.02em !important; }
                </style>';
                return true;
            }
            function css() {
                $v = file_exists(__DIR__ . '/adminer.css') ? filemtime(__DIR__ . '/adminer.css') : time();
                return ['adminer.css?v=' . $v];
            }
            function credentials() {
                $auth = $_SESSION['seintech_auth'] ?? [];
                if (!empty($auth['password'])) {
                    return [
                        $auth['server'] ?? '127.0.0.1',
                        $auth['username'] ?? '',
                        $auth['password'] ?? ''
                    ];
                }
                return array(SERVER, $_GET["username"] ?? "", get_password());
            }
            function database() {
                if (!empty($_GET['db'])) {
                    return (string)$_GET['db'];
                }
                $auth = $_SESSION['seintech_auth'] ?? [];
                if (!empty($auth['db'])) {
                    return (string)$auth['db'];
                }
                return parent::database();
            }
            function login($login, $password) {
                return true;
            }
        }
    }
    return new SeintechAdminer4();
}

// 5. Include Adminer core
if (file_exists(__DIR__ . '/adminer.php')) {
    include __DIR__ . '/adminer.php';
} elseif (file_exists('/usr/share/adminer/adminer.php')) {
    include '/usr/share/adminer/adminer.php';
} else {
    echo 'Adminer core binary missing. Please reinstall Adminer from Seintech Panel.';
}
