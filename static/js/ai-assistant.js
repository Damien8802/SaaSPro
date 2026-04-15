// Глобальная функция для открытия AI Assistant
window.openAIAssistant = function() {
    const widget = document.getElementById('aiWidget');
    if (widget) {
        widget.classList.toggle('open');
    }
};

// Добавляем кнопку AI Assistant на страницы
document.addEventListener('DOMContentLoaded', function() {
    // Если нет кнопки в навигации, добавляем плавающую кнопку
    if (!document.querySelector('.ai-fab')) {
        const fab = document.createElement('button');
        fab.className = 'ai-fab';
        fab.innerHTML = '<i class="fas fa-robot"></i>';
        fab.onclick = () => window.openAIAssistant();
        fab.style.cssText = `
            position: fixed;
            bottom: 30px;
            right: 30px;
            width: 60px;
            height: 60px;
            border-radius: 50%;
            background: linear-gradient(135deg, #667eea, #764ba2);
            color: white;
            border: none;
            cursor: pointer;
            z-index: 9999;
            box-shadow: 0 4px 12px rgba(0,0,0,0.2);
            font-size: 24px;
        `;
        document.body.appendChild(fab);
    }
});