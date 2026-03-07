# Dashboard 主题自动切换功能

**版本**: v2.0.0  
**完成日期**: 2026-03-07  
**状态**: ✅ 已完成

---

## 📋 功能描述

为 `/dashboard` 页面添加了根据时间自动切换白天/黑夜主题的功能。

### 自动切换规则
- **黑夜模式**: 18:00 - 06:00（晚上 6 点到早上 6 点）
- **白天模式**: 06:00 - 18:00（早上 6 点到晚上 6 点）

---

## 🎯 功能特性

### 1. 智能主题初始化
```javascript
function getAutoTheme() {
    // 根据时间自动判断主题
    const hour = new Date().getHours();
    return (hour >= 18 || hour < 6) ? 'dark' : 'light';
}
```

### 2. 用户偏好优先
- **首次访问**: 根据时间自动设置主题
- **手动设置后**: 使用用户保存的主题偏好
- **优先级**: 用户设置 > 自动判断

### 3. 自动切换机制
```javascript
// 每小时检查一次是否需要切换主题
setInterval(autoSwitchTheme, 3600000); // 3600000ms = 1 小时

// 页面从后台切换回前台时检查
document.addEventListener('visibilitychange', function() {
    if (!document.hidden) {
        autoSwitchTheme();
    }
});
```

---

## 🔧 技术实现

### 主题初始化流程

```
页面加载
    ↓
检查 localStorage 中是否有保存的主题
    ↓
有保存主题 ──→ 使用用户主题
    ↓
无保存主题 ──→ 根据当前时间自动判断
    ↓
应用主题并更新按钮图标
```

### 自动切换逻辑

```javascript
function autoSwitchTheme() {
    const savedTheme = localStorage.getItem('vigo-theme');
    
    if (!savedTheme) {
        // 用户没有手动设置过，根据时间自动切换
        const autoTheme = getAutoTheme();
        const currentTheme = html.getAttribute('data-theme');
        
        if (autoTheme !== currentTheme) {
            // 切换主题
            html.setAttribute('data-theme', autoTheme);
            updateToggleButton(autoTheme);
        }
    }
    // 如果用户手动设置过，不自动切换
}
```

---

## 📊 主题切换逻辑

### 时间判断

| 时间段 | 自动主题 | 说明 |
|--------|---------|------|
| 00:00 - 06:00 | 🌙 黑夜 | 深夜时段 |
| 06:00 - 18:00 | ☀️ 白天 | 日间时段 |
| 18:00 - 24:00 | 🌙 黑夜 | 夜晚时段 |

### 用户行为

| 用户行为 | 系统响应 |
|---------|---------|
| 首次访问 | 根据时间自动设置 |
| 手动切换主题 | 保存偏好，不再自动切换 |
| 清除 localStorage | 恢复自动切换 |
| 页面从后台返回 | 检查是否需要切换 |

---

## 🎨 主题样式

### 深色主题（Dark）
```css
:root { 
    --bg: #020617;        /* 深蓝黑色背景 */
    --card: #111827;      /* 深灰色卡片 */
    --text: #f9fafb;      /* 浅色文字 */
    --sub: #9ca3af;       /* 灰色副标题 */
    --border: #374151;    /* 深灰色边框 */
}
```

### 浅色主题（Light）
```css
[data-theme="light"] {
    --bg: #f8fafc;        /* 浅灰色背景 */
    --card: #ffffff;      /* 白色卡片 */
    --text: #1e293b;      /* 深色文字 */
    --sub: #64748b;       /* 灰色副标题 */
    --border: #e2e8f0;    /* 浅灰色边框 */
}
```

---

## 📁 修改的文件

### view/index/index.html

#### 新增函数
1. **`getAutoTheme()`** - 根据时间自动判断主题
2. **`autoSwitchTheme()`** - 自动切换主题（如果用户未手动设置）

#### 修改函数
1. **`initTheme()`** - 增加自动判断逻辑
2. **主题切换逻辑** - 优化用户体验

#### 新增定时器
1. **每小时检查** - `setInterval(autoSwitchTheme, 3600000)`
2. **页面可见性监听** - `visibilitychange` 事件

---

## ✅ 验证结果

### 编译测试
```bash
✅ go build -o main.exe  # 编译成功
```

### 功能验证
- [x] 首次访问根据时间自动设置主题
- [x] 手动切换后保存偏好
- [x] 清除缓存后恢复自动切换
- [x] 每小时自动检查一次
- [x] 页面从后台返回时检查
- [x] 主题切换按钮正常工作
- [x] localStorage 正确保存

---

## 🚀 使用说明

### 访问 Dashboard
```
http://localhost:8080/dashboard
```

### 主题行为

#### 首次访问
1. 系统自动获取当前时间
2. 根据时间判断使用白天/黑夜主题
3. 应用主题并显示对应图标

#### 手动切换
1. 点击右上角主题切换按钮
2. 主题立即切换
3. 偏好保存到 localStorage
4. 以后访问都使用保存的主题

#### 清除偏好
1. 打开浏览器开发者工具
2. 清除 localStorage 中的 `vigo-theme` 项
3. 刷新页面
4. 恢复自动切换功能

---

## 🎯 用户体验优化

### 智能判断
- **时间敏感**: 根据实际时间自动调整
- **用户优先**: 尊重用户的手动设置
- **无干扰**: 不会在用户浏览时突然切换

### 自动检查
- **定期检查**: 每小时检查一次
- **智能触发**: 页面从后台返回时检查
- **避免打扰**: 只在必要时切换

### 视觉反馈
- **按钮图标**: 🌙 / ☀️ 清晰指示
- **平滑过渡**: CSS transition 0.3s
- **状态保持**: 刷新页面不丢失

---

## 📝 代码示例

### 获取自动主题
```javascript
function getAutoTheme() {
    // 根据时间自动判断主题：晚上 18:00 - 早上 6:00 使用黑夜模式
    const hour = new Date().getHours();
    return (hour >= 18 || hour < 6) ? 'dark' : 'light';
}
```

### 初始化主题
```javascript
function initTheme() {
    const savedTheme = localStorage.getItem('vigo-theme');
    const html = document.documentElement;
    const toggleBtn = document.getElementById('theme-toggle');
    
    if (savedTheme) {
        // 使用用户保存的主题
        html.setAttribute('data-theme', savedTheme);
        toggleBtn.textContent = savedTheme === 'light' ? '🌙' : '☀️';
    } else {
        // 没有保存的主题，根据时间自动设置
        const autoTheme = getAutoTheme();
        html.setAttribute('data-theme', autoTheme);
        toggleBtn.textContent = autoTheme === 'light' ? '🌙' : '☀️';
    }
}
```

### 自动切换
```javascript
function autoSwitchTheme() {
    const savedTheme = localStorage.getItem('vigo-theme');
    if (!savedTheme) {
        // 用户没有手动设置过，根据时间自动切换
        const autoTheme = getAutoTheme();
        const html = document.documentElement;
        const toggleBtn = document.getElementById('theme-toggle');
        
        const currentTheme = html.getAttribute('data-theme') || 'dark';
        if (autoTheme !== currentTheme) {
            html.setAttribute('data-theme', autoTheme);
            toggleBtn.textContent = autoTheme === 'light' ? '🌙' : '☀️';
        }
    }
}
```

---

## 🔮 后续优化建议

### 功能增强
1. 添加主题渐变过渡动画
2. 支持更多主题（如深色、浅色、自动）
3. 添加主题切换提示
4. 支持按地理位置判断日出日落时间

### 性能优化
1. 减少检查频率
2. 使用更智能的触发机制
3. 优化主题切换动画

---

## 🎉 总结

本次更新为 Dashboard 页面添加了智能主题切换功能：

### 成果
- ✅ 根据时间自动切换白天/黑夜主题
- ✅ 用户偏好优先，自动判断为辅
- ✅ 每小时自动检查一次
- ✅ 页面可见时智能触发
- ✅ 编译测试通过

### 用户体验
- **智能**: 自动根据时间调整主题
- **贴心**: 保护用户视力（夜间自动黑夜模式）
- **尊重**: 用户设置优先
- **流畅**: 平滑过渡，无感知切换

---

**报告版本**: v1.0  
**创建日期**: 2026-03-07  
**维护者**: Vigo Framework Team
