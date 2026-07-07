package com.rkhcfs.go;

import android.app.Activity;
import android.app.AlertDialog;
import android.graphics.Color;
import android.os.Bundle;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.text.TextUtils;
import android.view.Gravity;
import android.view.Window;
import android.view.WindowManager;
import android.view.ViewGroup;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileOutputStream;
import java.io.FileReader;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.URL;
import java.text.DecimalFormat;
import java.util.Arrays;
import java.util.Comparator;

public class MainActivity extends Activity {
    private static final String WEB_URL = "http://127.0.0.1:8080";
    private WebView webView;
    private FrameLayout root;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private static final Object BACKEND_LOCK = new Object();
    private static Process backendProcess;
    private static volatile boolean backendStarted = false;
    private static volatile boolean backendStarting = false;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureSystemBars();
        buildUi();
        startBackend();
    }

    private void configureSystemBars() {
        Window window = getWindow();
        window.addFlags(WindowManager.LayoutParams.FLAG_DRAWS_SYSTEM_BAR_BACKGROUNDS);
        window.clearFlags(WindowManager.LayoutParams.FLAG_TRANSLUCENT_STATUS);
        window.clearFlags(WindowManager.LayoutParams.FLAG_TRANSLUCENT_NAVIGATION);
        window.setStatusBarColor(Color.BLACK);
        window.setNavigationBarColor(Color.BLACK);

        // Keep the complete WebView below the status/notification bar. The
        // API-35 theme also opts out of forced edge-to-edge, so the FA/EN
        // selector can never sit under notification icons or a display cutout.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            window.setDecorFitsSystemWindows(true);
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            window.setStatusBarContrastEnforced(false);
            window.setNavigationBarContrastEnforced(false);
        }
    }

    private void buildUi() {
        root = new FrameLayout(this);
        root.setBackgroundColor(Color.BLACK);

        webView = new WebView(this);
        webView.setBackgroundColor(Color.BLACK);
        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setLoadWithOverviewMode(true);
        settings.setUseWideViewPort(true);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        // The UI and header icon are bundled with the APK. Never reuse an old
        // WebView copy after an app update.
        settings.setCacheMode(WebSettings.LOAD_NO_CACHE);
        settings.setSupportZoom(false);

        webView.clearCache(true);
        webView.addJavascriptInterface(new NativeBridge(), "SimorghNative");
        webView.setWebViewClient(new WebViewClient());
        root.addView(webView, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT
        ));

        setContentView(root);
    }

    private void addFloatingButtons() {
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setGravity(Gravity.CENTER);
        box.setPadding(0, 0, 18, 28);

        Button logs = makeFloatButton("Logs");
        logs.setOnClickListener(v -> showDiagnosticsDialog(""));

        Button web = makeFloatButton("Web");
        web.setOnClickListener(v -> loadPanel());

        box.addView(logs);
        box.addView(web);

        FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT,
                ViewGroup.LayoutParams.WRAP_CONTENT,
                Gravity.BOTTOM | Gravity.RIGHT
        );
        root.addView(box, lp);
    }

    private Button makeFloatButton(String text) {
        Button b = new Button(this);
        b.setText(text);
        b.setTextColor(Color.WHITE);
        b.setTextSize(11);
        b.setAllCaps(false);
        b.setBackgroundColor(Color.argb(220, 0, 245, 255));
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(dp(74), dp(42));
        lp.setMargins(0, 0, 0, 10);
        b.setLayoutParams(lp);
        return b;
    }

    private int dp(int v) {
        return (int) (v * getResources().getDisplayMetrics().density);
    }

    private void startBackend() {
        showLoading();
        ensureBackendRunning(true);
    }

    private void ensureBackendRunning(boolean loadWhenReady) {
        new Thread(() -> {
            if (isBackendReady()) {
                backendStarted = true;
                if (loadWhenReady) handler.post(this::loadPanel);
                return;
            }

            boolean launchBackend = false;
            synchronized (BACKEND_LOCK) {
                if (!isProcessAlive(backendProcess) && !backendStarting) {
                    backendStarting = true;
                    launchBackend = true;
                }
            }

            if (launchBackend) {
                try {
                    File base = new File(getFilesDir(), "rkhcfs");
                    deleteRecursively(new File(base, "ip-ranges"));
                    deleteRecursively(new File(base, "isp-data"));
                    deleteRecursively(new File(base, "r"));
                    copyAssetFolder("rkhcfs", base);

                    File go = new File(getApplicationInfo().nativeLibraryDir, "librkhcfs_go.so");
                    File xray = new File(getApplicationInfo().nativeLibraryDir, "libxray.so");
                    if (!go.exists()) throw new Exception("Go backend binary missing: " + go.getAbsolutePath());

                    ProcessBuilder pb = new ProcessBuilder(go.getAbsolutePath());
                    pb.environment().put("RKH_BASE_DIR", base.getAbsolutePath());
                    pb.environment().put("RKH_XRAY_EXEC", xray.getAbsolutePath());
                    pb.redirectErrorStream(true);
                    Process started = pb.start();
                    synchronized (BACKEND_LOCK) {
                        backendProcess = started;
                    }
                    drainBackendOutput(started);
                    monitorBackendProcess(started);
                } catch (Exception e) {
                    backendStarted = false;
                    if (loadWhenReady) {
                        handler.post(() -> showStartupError("Backend error: " + e.getMessage()));
                    }
                    return;
                } finally {
                    backendStarting = false;
                }
            }

            waitForBackendAndLoad(loadWhenReady);
        }).start();
    }

    private void waitForBackendAndLoad(boolean loadWhenReady) {
        boolean ready = false;
        for (int i = 0; i < 60; i++) {
            if (isBackendReady()) {
                ready = true;
                break;
            }
            try {
                Thread.sleep(300);
            } catch (InterruptedException ignored) {
                Thread.currentThread().interrupt();
                break;
            }
        }

        backendStarted = ready;
        if (ready) {
            if (loadWhenReady) handler.post(this::loadPanel);
        } else if (loadWhenReady) {
            handler.post(() -> showStartupError("Backend did not become ready. Tap Logs to inspect diagnostics."));
        }
    }

    private boolean isProcessAlive(Process process) {
        if (process == null) return false;
        try {
            process.exitValue();
            return false;
        } catch (IllegalThreadStateException running) {
            return true;
        }
    }

    private void drainBackendOutput(Process process) {
        new Thread(() -> {
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
                while (reader.readLine() != null) {
                    // Drain the pipe so the native backend can never block on stdout/stderr.
                }
            } catch (Exception ignored) {}
        }, "simorgh-backend-output").start();
    }

    private void monitorBackendProcess(Process process) {
        new Thread(() -> {
            try {
                process.waitFor();
            } catch (InterruptedException ignored) {
                Thread.currentThread().interrupt();
            } finally {
                synchronized (BACKEND_LOCK) {
                    if (backendProcess == process) backendProcess = null;
                }
                backendStarted = false;
            }
        }, "simorgh-backend-monitor").start();
    }

    private final class NativeBridge {
        @JavascriptInterface
        public void ensureBackend() {
            ensureBackendRunning(false);
        }
    }

    private boolean isBackendReady() {
        HttpURLConnection conn = null;
        try {
            URL u = new URL(WEB_URL + "/api/health");
            conn = (HttpURLConnection) u.openConnection();
            conn.setConnectTimeout(1000);
            conn.setReadTimeout(1500);
            conn.setRequestMethod("GET");
            int code = conn.getResponseCode();
            return code >= 200 && code < 500;
        } catch (Exception ignored) {
            return false;
        } finally {
            if (conn != null) conn.disconnect();
        }
    }

    private void loadPanel() {
        webView.loadUrl(WEB_URL + "/?app=047");
    }

    private void showLoading() {
        String html =
                "<html><head><meta name='viewport' content='width=device-width,initial-scale=1'>" +
                "<style>" +
                "@font-face{font-family:SuperPixel;src:url('fonts/SuperPixel-m2L8j.ttf')}@font-face{font-family:PsygenDemo;src:url('fonts/PsygenDemo-AREoM.otf')}" +
                "body{margin:0;min-height:100vh;background:radial-gradient(circle at 20% 18%,#031b24,#000 52%,#05000a);color:#fff;font-family:PsygenDemo,monospace;display:flex;align-items:center;justify-content:center;overflow:hidden}" +
                ".card{width:min(86vw,430px);background:linear-gradient(145deg,rgba(0,245,255,.12),rgba(255,43,214,.08)),rgba(5,6,14,.94);border:1px solid rgba(0,245,255,.32);border-radius:30px;padding:30px 22px;text-align:center;box-shadow:0 28px 80px rgba(0,0,0,.62),inset 0 0 42px rgba(0,245,255,.06);position:relative}" +
                ".badge{display:inline-flex;align-items:center;gap:8px;padding:7px 12px;border-radius:999px;background:rgba(0,245,255,.10);border:1px solid rgba(0,245,255,.28);color:#bafcff;font-weight:800;font-size:12px}" +
                ".lamp{width:8px;height:8px;border-radius:50%;background:#00f5ff;box-shadow:0 0 10px #00f5ff}" +
                "h1{margin:20px 0 8px;color:#00f5ff;font-family:SuperPixel,monospace;font-size:36px;font-weight:400;letter-spacing:.5px;text-shadow:0 0 28px rgba(255,45,145,.38)}" +
                ".sub{margin:0;color:#cbd5ee;font-size:13px}.tag{margin-top:22px;color:#b86cff;font-weight:900}" +
                ".loader{width:170px;height:8px;border-radius:999px;background:rgba(255,255,255,.08);margin:26px auto 4px;overflow:hidden;border:1px solid rgba(0,245,255,.18)}" +
                ".loader span{display:block;width:100%;height:100%;border-radius:999px;background:linear-gradient(90deg,#00f5ff,#b86cff);box-shadow:0 0 12px rgba(0,245,255,.35)}" +
                ".dots{display:none}" +
                "" +
                "</style></head>" +
                "<body><div class='card'><div class='badge'><span class='lamp'></span>SIMORGH Scanner Android</div><h1>SIMORGH Scanner</h1><p class='sub'>Starting real Xray scanner engine</p><div class='loader'><span></span></div><div class='dots'><i></i><i></i><i></i></div></div></body></html>";
        webView.loadDataWithBaseURL("file:///android_asset/rkhcfs/", html, "text/html", "UTF-8", null);
    }


    private void showStartupError(String message) {
        String safe = message == null ? "Unknown error" : message
                .replace("&", "&amp;")
                .replace("<", "&lt;")
                .replace(">", "&gt;");

        String html =
                "<html><head><meta name='viewport' content='width=device-width,initial-scale=1'>" +
                "<style>" +
                "@font-face{font-family:SuperPixel;src:url('fonts/SuperPixel-m2L8j.ttf')}@font-face{font-family:PsygenDemo;src:url('fonts/PsygenDemo-AREoM.otf')}" +
                "body{margin:0;min-height:100vh;background:radial-gradient(circle at 20% 18%,#031b24,#000 52%,#05000a);color:#fff;font-family:PsygenDemo,monospace;display:flex;align-items:center;justify-content:center;overflow:hidden}" +
                ".card{width:min(86vw,460px);background:linear-gradient(145deg,rgba(0,245,255,.12),rgba(255,43,214,.08)),rgba(5,6,14,.94);border:1px solid rgba(0,245,255,.32);border-radius:30px;padding:28px 22px;text-align:center;box-shadow:0 28px 80px rgba(0,0,0,.62)}" +
                "h1{margin:0 0 10px;color:#00f5ff;font-family:SuperPixel,monospace;font-size:32px;font-weight:400;text-shadow:0 0 28px rgba(255,45,145,.38)}" +
                ".msg{margin:14px 0;color:#dffbff;font-size:13px;line-height:1.55;background:rgba(0,245,255,.08);border:1px solid rgba(0,245,255,.20);border-radius:16px;padding:12px}" +
                ".hint{color:#b86cff;font-weight:800;font-size:13px;margin-top:12px}" +
                "</style></head>" +
                "<body><div class='card'><h1>SIMORGH Scanner</h1><div class='msg'>" + safe + "</div><div class='hint'>Use the Logs button only if you want diagnostics.</div></div></body></html>";
        webView.loadDataWithBaseURL("file:///android_asset/rkhcfs/", html, "text/html", "UTF-8", null);
    }

    private void showDiagnosticsDialog(String extraMessage) {
        String plain = buildDiagnosticsPlainText(extraMessage);
        TextView tv = new TextView(this);
        tv.setText(plain);
        tv.setTextColor(Color.rgb(210, 252, 255));
        tv.setTextSize(12);
        tv.setPadding(dp(14), dp(14), dp(14), dp(14));
        tv.setTextIsSelectable(true);

        ScrollView sv = new ScrollView(this);
        sv.setBackgroundColor(Color.BLACK);
        sv.addView(tv);

        AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("SIMORGH Scanner Logs / Diagnostics")
                .setView(sv)
                .setPositiveButton("Close", null)
                .setNegativeButton("Refresh", null)
                .create();

        dialog.setOnShowListener(d -> dialog.getButton(AlertDialog.BUTTON_NEGATIVE)
                .setOnClickListener(v -> tv.setText(buildDiagnosticsPlainText(extraMessage))));
        dialog.show();
    }

    private String buildDiagnosticsPlainText(String extraMessage) {
        File base = new File(getFilesDir(), "rkhcfs");
        File go = new File(getApplicationInfo().nativeLibraryDir, "librkhcfs_go.so");
        File xray = new File(getApplicationInfo().nativeLibraryDir, "libxray.so");
        File geoip = new File(base, "geoip.dat");
        File geosite = new File(base, "geosite.dat");
        File latestJobLog = findLatestJobLog(base);

        StringBuilder sb = new StringBuilder();
        sb.append("Backend started: ").append(backendStarted ? "FOUND" : "NOT CONFIRMED").append("\n\n");
        if (!TextUtils.isEmpty(extraMessage)) sb.append("Message: ").append(extraMessage).append("\n\n");
        sb.append(fileLine("Backend", go, true));
        sb.append(fileLine("xray native binary", xray, true));
        sb.append(fileLine("geoip.dat", geoip, false));
        sb.append(fileLine("geosite.dat", geosite, false));

        sb.append("\n--- Latest job.log ---\n");
        sb.append(latestJobLog == null ? "No job log found yet." : latestJobLog.getAbsolutePath()).append("\n");
        sb.append(readTail(latestJobLog, 24000));

        sb.append("\n\n--- Job folders ---\n");
        sb.append(listJobs(base));
        return sb.toString();
    }

    private String fileLine(String title, File file, boolean executable) {
        if (file == null) return title + ": MISSING\n\n";
        return title + ": " + (file.exists() ? "FOUND" : "MISSING") +
                "\n  " + file.getAbsolutePath() +
                "\n  Size: " + fmt(file.length()) +
                (executable ? "\n  canExecute: " + file.canExecute() : "") + "\n\n";
    }

    private File findLatestJobLog(File base) {
        File jobs = new File(base, "web_runtime/jobs");
        File[] dirs = jobs.listFiles();
        if (dirs == null || dirs.length == 0) return null;
        Arrays.sort(dirs, Comparator.comparingLong(File::lastModified).reversed());
        for (File d : dirs) {
            File log = new File(d, "job.log");
            if (log.exists()) return log;
        }
        return null;
    }

    private String listJobs(File base) {
        File jobs = new File(base, "web_runtime/jobs");
        File[] dirs = jobs.listFiles();
        if (dirs == null || dirs.length == 0) return "No jobs yet.";
        Arrays.sort(dirs, Comparator.comparingLong(File::lastModified).reversed());
        StringBuilder sb = new StringBuilder();
        for (File d : dirs) {
            File log = new File(d, "job.log");
            sb.append(d.getName()).append("  log=").append(log.exists() ? fmt(log.length()) : "missing").append("\n");
        }
        return sb.toString();
    }

    private String readTail(File file, int maxChars) {
        if (file == null || !file.exists()) return "Not found.";
        StringBuilder sb = new StringBuilder();
        try (BufferedReader br = new BufferedReader(new FileReader(file))) {
            String line;
            while ((line = br.readLine()) != null) {
                sb.append(line).append("\n");
                if (sb.length() > maxChars * 2) sb.delete(0, sb.length() - maxChars);
            }
        } catch (Exception e) {
            return "Read error: " + e.getMessage();
        }
        if (sb.length() > maxChars) return sb.substring(sb.length() - maxChars);
        return sb.toString();
    }

    private String fmt(long bytes) {
        if (bytes <= 0) return "0 B";
        double v = bytes;
        String[] units = {"B", "KB", "MB", "GB"};
        int idx = 0;
        while (v >= 1024 && idx < units.length - 1) {
            v /= 1024;
            idx++;
        }
        return new DecimalFormat("#.##").format(v) + " " + units[idx];
    }

    private void copyAssetFolder(String assetPath, File destDir) throws Exception {
        String[] items = getAssets().list(assetPath);
        if (items == null) return;
        if (!destDir.exists()) destDir.mkdirs();
        for (String item : items) {
            String childAssetPath = assetPath + "/" + item;
            File childDest = new File(destDir, item);
            String[] children = getAssets().list(childAssetPath);
            if (children != null && children.length > 0) copyAssetFolder(childAssetPath, childDest);
            else copyAssetFile(childAssetPath, childDest);
        }
    }

    private void copyAssetFile(String assetPath, File destFile) throws Exception {
        if (destFile.exists() && destFile.length() > 0 && !shouldOverwriteAsset(assetPath)) return;
        File parent = destFile.getParentFile();
        if (parent != null && !parent.exists()) parent.mkdirs();
        try (InputStream in = getAssets().open(assetPath); FileOutputStream out = new FileOutputStream(destFile)) {
            byte[] buffer = new byte[1024 * 64];
            int read;
            while ((read = in.read(buffer)) != -1) out.write(buffer, 0, read);
        }
    }

    private boolean shouldOverwriteAsset(String assetPath) {
        return assetPath.endsWith("/index.html")
                || assetPath.endsWith("/simorgh_icon.png")
                || assetPath.contains("/fonts/")
                || assetPath.contains("/r/");
    }


    private void deleteRecursively(File file) {
        if (file == null || !file.exists()) return;
        if (file.isDirectory()) {
            File[] children = file.listFiles();
            if (children != null) {
                for (File child : children) deleteRecursively(child);
            }
        }
        //noinspection ResultOfMethodCallIgnored
        file.delete();
    }

    @Override
    protected void onResume() {
        super.onResume();
        // Returning to the Activity or switching Android UI state must not
        // reset the web session. Wake the existing poll loop and verify that
        // the native backend is still available without reloading the page.
        ensureBackendRunning(false);
        if (webView != null) {
            webView.post(() -> webView.evaluateJavascript(
                    "window.simorghOnHostResume && window.simorghOnHostResume()",
                    null
            ));
        }
    }

    @Override
    protected void onDestroy() {
        // The backend belongs to the application process, not to one Activity
        // instance. Keeping it alive prevents transient "Failed to fetch"
        // errors when Android recreates or relaunches the Activity.
        super.onDestroy();
    }

    @SuppressWarnings("deprecation")
    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) webView.goBack();
        else super.onBackPressed();
    }
}