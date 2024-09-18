// app.js

const app = document.getElementById('app');

const loadContent = async (path) => {
    try {
        const response = await fetch(`/__spa__${path}`);
        if (!response.ok) throw new Error('Page not found');
        
        const data = await response.json();
        app.innerHTML = data.Content;
        
        // Apply styles
        const styleElement = document.createElement('style');
        styleElement.textContent = data.Style;
        document.head.appendChild(styleElement);
        
        // Execute script
        const scriptElement = document.createElement('script');
        scriptElement.textContent = data.Script;
        document.body.appendChild(scriptElement);
        
    } catch (error) {
        console.error('Error loading content:', error);
        app.innerHTML = '<h1>404 - Page Not Found</h1>';
    }
};

const handleNavigation = (e) => {
    if (!window.isSPAMode) return; // Allow default navigation if SPA mode is off
    
    e = e || window.event;
    e.preventDefault();
    window.history.pushState({}, "", e.target.href);
    loadContent(e.target.pathname);
};

if (window.isSPAMode) {
    document.body.addEventListener('click', (e) => {
        if (e.target.tagName === 'A' && e.target.origin === window.location.origin) {
            handleNavigation(e);
        }
    });

    window.addEventListener('popstate', () => {
        loadContent(window.location.pathname);
    });
}