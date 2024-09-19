// app.js

import { TransitionManager } from './transition-manager.js';

if (window.isSPAMode) {
    const transitionManager = new TransitionManager();
    transitionManager.init();

    // Register a custom transition for the movie page
    // transitionManager.registerCustomTransition('/movie', (context) => {
    //     const { clickedElement, clickedRect, targetPath } = context;
        
    //     // Find the clicked thumbnail
    //     const thumbnail = clickedElement.closest('.thumbnail');
    //     if (!thumbnail) return; // Exit if not a thumbnail click

    //     const movieId = thumbnail.dataset.movieId;
        
    //     // Find or create the full content element
    //     let fullContent = document.querySelector(`.full-content[data-movie-id="${movieId}"]`);
    //     if (!fullContent) {
    //         fullContent = document.createElement('div');
    //         fullContent.className = 'full-content';
    //         fullContent.dataset.movieId = movieId;
    //         document.body.appendChild(fullContent);
    //     }

    //     // Set up the transition
    //     thumbnail.style.viewTransitionName = `thumbnail-${movieId}`;
    //     fullContent.style.viewTransitionName = `thumbnail-${movieId}`;

    //     // Position the full content initially at the thumbnail's position
    //     fullContent.style.position = 'fixed';
    //     fullContent.style.top = `${clickedRect.top}px`;
    //     fullContent.style.left = `${clickedRect.left}px`;
    //     fullContent.style.width = `${clickedRect.width}px`;
    //     fullContent.style.height = `${clickedRect.height}px`;

    //     // Force a reflow
    //     fullContent.offsetHeight;

    //     // Animate to full screen
    //     fullContent.style.top = '0';
    //     fullContent.style.left = '0';
    //     fullContent.style.width = '100%';
    //     fullContent.style.height = '100%';

    //     // You might want to load the actual content here or in a subsequent step
    // });

} else {
    console.log("Running in non-SPA mode");
}