// ui/static/js/dashboard.js

// --- Flash Message Closing Logic ---

// Function to handle closing flash messages using event delegation
function setupFlashCloseListener() {
    // Find a stable parent container that always exists on the dashboard page.
    // '.dashboard-main' seems appropriate as it contains the content area where flashes appear.
    const dashboardMain = document.querySelector('.dashboard-main');

    // If the container doesn't exist on the page, exit (safety check)
    if (!dashboardMain) {
        console.warn("Dashboard main container not found, cannot attach flash close listener.");
        return;
    }

    // --- Event Delegation: Add ONE click listener to the stable parent ---
    // Check if listener already attached to prevent duplicates if this function is called multiple times
    if (!dashboardMain.dataset.flashListenerAttached) {
        dashboardMain.addEventListener('click', function (event) {
            // Check if the element that was actually clicked (event.target)
            // is our close button or is inside our close button (e.g., the 'x' symbol itself)
            const closeButton = event.target.closest('.flash-close-btn');

            if (closeButton) {
                console.log("Flash close button clicked!"); // For debugging

                // Find the closest ancestor element with the class 'flash-message'
                const flashMessage = closeButton.closest('.flash-message');

                // If found, remove it from the page with a fade effect
                if (flashMessage) {
                    flashMessage.style.transition = 'opacity 0.3s ease-out, margin-top 0.3s ease-out'; // Add margin transition
                    flashMessage.style.opacity = '0';
                    flashMessage.style.marginTop = `-${flashMessage.offsetHeight}px`; // Slide up effect (optional)

                    // Remove the element after the transition completes
                    setTimeout(() => {
                        flashMessage.remove();
                        console.log("Flash message removed."); // For debugging
                    }, 300); // Match timeout to transition duration
                } else {
                    console.warn("Could not find parent flash message element for clicked button."); // For debugging
                }
            }
        });
        // Mark that the listener has been attached
        dashboardMain.dataset.flashListenerAttached = 'true';
        console.log("Flash close listener attached to .dashboard-main"); // For debugging
    } else {
        console.log("Flash close listener already attached."); // Debug if setup called again
    }
}


// --- View More & Modal Logic (User's existing code) ---

// Function to check for overflow and show/hide 'View More' links
function checkAndSetupViewMore() {
    console.log("Running checkAndSetupViewMore...");
    const moodItems = document.querySelectorAll('.mood-item');

    if (moodItems.length === 0) {
        console.log("No mood items found to process.");
        return;
    }

    moodItems.forEach(item => {
        const contentContainer = item.querySelector('.quill-rendered-content');
        const viewMoreLink = item.querySelector('.view-more-link');

        if (!contentContainer || !viewMoreLink) {
            console.warn('Could not find content container or view more link for item:', item.id);
            return;
        }

        // Use a small timeout to allow the browser to render and calculate heights
        setTimeout(() => {
            const isOverflowing = contentContainer.scrollHeight > contentContainer.clientHeight + 1;

            if (isOverflowing) {
                viewMoreLink.style.display = 'inline';
                viewMoreLink.removeEventListener('click', handleViewMoreClick);
                viewMoreLink.addEventListener('click', handleViewMoreClick);
            } else {
                viewMoreLink.style.display = 'none';
                viewMoreLink.removeEventListener('click', handleViewMoreClick);
            }
        }, 50);
    });
}

// Function to handle clicking the 'View More' link
function handleViewMoreClick(event) {
    event.preventDefault();
    const link = event.currentTarget;
    const modal = document.getElementById('mood-detail-modal');

    if (!modal) {
        console.error("Mood detail modal element not found!");
        return;
    }

    const title = link.dataset.title || 'Mood Entry';
    const emotion = link.dataset.emotion || 'Unknown';
    const emoji = link.dataset.emoji || '❓';
    const createdAt = link.dataset.createdAt || 'N/A';
    const fullContent = link.dataset.fullContent || '<p>Content not available.</p>';

    console.log("Retrieved fullContent for modal:", fullContent);

    const modalTitleEl = modal.querySelector('#modal-title');
    const modalEmojiEl = modal.querySelector('#modal-emoji');
    const modalEmotionNameEl = modal.querySelector('#modal-emotion-name');
    const modalCreatedAtEl = modal.querySelector('#modal-created-at');
    const modalFullContentEl = modal.querySelector('#modal-full-content');

    if (modalTitleEl) modalTitleEl.textContent = title;
    else console.error("Modal title element not found");
    if (modalEmojiEl) modalEmojiEl.textContent = emoji;
    else console.error("Modal emoji element not found");
    if (modalEmotionNameEl) modalEmotionNameEl.textContent = emotion;
    else console.error("Modal emotion name element not found");
    if (modalCreatedAtEl) modalCreatedAtEl.textContent = createdAt;
    else console.error("Modal created at element not found");
    if (modalFullContentEl) modalFullContentEl.innerHTML = fullContent;
    else console.error("Modal full content element not found");

    modal.classList.add('is-visible');
    document.body.style.overflow = 'hidden';
}

// Function to set up modal close functionality
function setupModalClose() {
    const modal = document.getElementById('mood-detail-modal');
    if (!modal) {
        return;
    }

    const closeButton = modal.querySelector('#modal-detail-close-button');
    if (!closeButton) {
        console.error('Modal close button not found!');
        return;
    }

    const closeModal = () => {
        const currentModal = document.getElementById('mood-detail-modal');
        if (currentModal && currentModal.classList.contains('is-visible')) {
             currentModal.classList.remove('is-visible');
             document.body.style.overflow = '';
        }
    };

    // Use dataset flags to ensure listeners are attached only once
    if (!closeButton.dataset.listenerAttached) {
         closeButton.addEventListener('click', closeModal);
         closeButton.dataset.listenerAttached = 'true';
    }
    if (!modal.dataset.overlayListenerAttached) {
        modal.addEventListener('click', (event) => {
            if (event.target === modal) {
                closeModal();
            }
        });
         modal.dataset.overlayListenerAttached = 'true';
     }
    if (!document.body.dataset.escapeListenerAttached) {
        document.addEventListener('keydown', (event) => {
            const currentModal = document.getElementById('mood-detail-modal');
            if (event.key === 'Escape' && currentModal && currentModal.classList.contains('is-visible')) {
                closeModal();
            }
        });
         document.body.dataset.escapeListenerAttached = 'true';
     }
}

// --- Initialization ---

// Initial Setup on Page Load
document.addEventListener('DOMContentLoaded', () => {
    console.log("DOM fully loaded and parsed.");
    setupFlashCloseListener(); // <-- Initialize flash close listener
    checkAndSetupViewMore();   // Check existing items on load
    setupModalClose();         // Set up modal close listeners once
});

// Setup after HTMX Content Swaps
document.body.addEventListener('htmx:afterSwap', function(event) {
    const targetArea = document.getElementById('dashboard-content-area');
    // Check if the swap target is relevant (contains mood items or is the dashboard area)
    if (targetArea && (targetArea.contains(event.detail.target) || event.detail.target === targetArea || event.detail.target.querySelector('.mood-item'))) {
        console.log('htmx:afterSwap detected - Re-running View More setup.');
        checkAndSetupViewMore(); // Re-check for overflow on newly added/swapped items

        // Note: setupFlashCloseListener doesn't need to be called here
        // because the listener is attached to the parent (.dashboard-main)
        // which isn't swapped by HTMX in this setup. Event delegation handles new elements.

        // Similarly, setupModalClose only needs to run once on initial load
        // unless the modal itself is part of the HTMX swap.
    }
});