// public/app.js

const app = document.getElementById('app');
let isSPAMode = false;

const routes = {
    '/': 'home',
    '/about': 'about',
    '/contact': 'contact'
};

const loadContent = async (path) => {
    try {
        const response = await fetch(isSPAMode ? `/api${path}` : path, {
            headers: {
                'X-Requested-With': 'XMLHttpRequest'
            }
        });
        if (!response.ok) throw new Error('Page not found');
        const data = await response.json();
        
        app.innerHTML = `
            ${data.Content}
            <style>${data.Style}</style>
        `;
        
        const script = document.createElement('script');
        script.textContent = data.Script;
        document.body.appendChild(script);
        
        // Update the page title
        document.title = `Mehndi Masterpiece - ${path.charAt(1).toUpperCase() + path.slice(2)}`;
        
    } catch (error) {
        console.error('Error loading content:', error);
        app.innerHTML = '<h1>404 - Page Not Found</h1>';
    }
};

const handleNavigation = (e) => {
    if (!isSPAMode) return; // Let the default navigation happen in non-SPA mode
    
    e = e || window.event;
    e.preventDefault();
    window.history.pushState({}, "", e.target.href);
    handleLocation();
};

const handleLocation = () => {
    const path = window.location.pathname;
    const route = routes[path] || path.slice(1);
    loadContent(`/${route}`);
};

window.onpopstate = handleLocation;
window.route = handleNavigation;

// Toggle SPA mode
const toggleSPA = () => {
    isSPAMode = !isSPAMode;
    localStorage.setItem('spaMode', isSPAMode);
    document.body.classList.toggle('spa-mode', isSPAMode);
    console.log(`SPA mode ${isSPAMode ? 'enabled' : 'disabled'}`);
};

// Initialize based on saved preference
isSPAMode = localStorage.getItem('spaMode') === 'true';
document.body.classList.toggle('spa-mode', isSPAMode);

// Only handle location if in SPA mode
if (isSPAMode) {
    handleLocation();
}