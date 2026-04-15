// Автоматическая загрузка AI Assistant на все страницы
(function() {
    // Проверяем, не загружен ли уже
    if (document.getElementById('aiAssistantFull')) return;
    
    // Загружаем скрипт виджета
    const script = document.createElement('script');
    script.src = '/static/ai-assistant-full.js';
    script.async = true;
    document.head.appendChild(script);
})();