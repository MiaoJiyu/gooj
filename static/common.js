// Gooj 通用 JavaScript 函数

// 显示提示信息
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.textContent = message;
    toast.style.cssText = 'position:fixed;top:20px;right:20px;padding:12px 24px;border-radius:8px;color:white;font-size:14px;z-index:10000;animation:slideIn 0.3s ease;box-shadow:0 4px 12px rgba(0,0,0,0.15);background:#4a90d9;';
    document.body.appendChild(toast);
    setTimeout(function() {
        toast.style.animation = 'slideOut 0.3s ease';
        setTimeout(function() { toast.remove(); }, 300);
    }, 2500);
}

// 复制文本到剪贴板
async function copyToClipboard(text, btn) {
    try {
        await navigator.clipboard.writeText(text);
        if (btn) {
            btn.classList.add('copied');
            var originalText = btn.innerHTML;
            btn.innerHTML = 'copied';
            setTimeout(function() {
                btn.classList.remove('copied');
                btn.innerHTML = originalText;
            }, 1500);
        }
    } catch (err) {
        var textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        try {
            document.execCommand('copy');
            showToast('copied', 'success');
        } catch (e) {
            showToast('copy failed', 'error');
        }
        document.body.removeChild(textarea);
    }
}

// 为代码块添加复制按钮
function initCodeCopyButtons() {
    document.querySelectorAll('pre').forEach(function(pre) {
        if (pre.querySelector('.copy-btn')) return;
        var btn = document.createElement('button');
        btn.className = 'copy-btn';
        btn.innerHTML = 'copy';
        btn.onclick = function(e) {
            e.stopPropagation();
            var code = pre.querySelector('code') ? pre.querySelector('code').textContent : pre.textContent;
            copyToClipboard(code, btn);
        };
        pre.style.position = 'relative';
        pre.appendChild(btn);
    });
}

// 加载用户信息到导航栏
async function loadNavbarUserInfo() {
    var username = localStorage.getItem('username');
    var userInfoEl = document.getElementById('navbarUser');
    var ratingEl = document.getElementById('navbarRating');
    
    if (!username) {
        if (userInfoEl) userInfoEl.textContent = 'not logged in';
        return;
    }
    
    if (userInfoEl) userInfoEl.textContent = username;
    
    try {
        var res = await fetch('/api/user/' + encodeURIComponent(username));
        if (res.ok) {
            var data = await res.json();
            if (ratingEl && data.rating) {
                ratingEl.textContent = data.rating;
            }
        }
    } catch (e) {
        console.error('Failed to load user rating:', e);
    }
}

// 登出函数
function logout() {
    document.cookie = "token=; Max-Age=0; path=/";
    localStorage.removeItem('username');
    localStorage.removeItem('login_ts');
    localStorage.removeItem('auth_token');
    location.href = '/';
}

// 添加动画样式
var styleSheet = document.createElement('style');
styleSheet.textContent = '@keyframes slideIn{from{transform:translateX(100%);opacity:0;}to{transform:translateX(0);opacity:1;}}@keyframes slideOut{from{transform:translateX(0);opacity:1;}to{transform:translateX(100%);opacity:0;}}';
document.head.appendChild(styleSheet);

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    loadNavbarUserInfo();
});
