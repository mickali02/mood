// ui/static/js/dashboard.js

// Function to check for overflow and show/hide 'View More' links
function checkAndSetupViewMore() {
    console.log("Running checkAndSetupViewMore..."); // Debug log
    const moodItems = document.querySelectorAll('.mood-item');

    if (moodItems.length === 0) {
        console.log("No mood items found to process."); // Debug log
        return;
    }

    moodItems.forEach(item => {
        const contentContainer = item.querySelector('.quill-rendered-content');
        const viewMoreLink = item.querySelector('.view-more-link');

        if (!contentContainer || !viewMoreLink) {
            console.warn('Could not find content container or view more link for item:', item.id); // Debug log
            return; // Skip if elements are missing
        }

        // Use a small timeout to allow the browser to render and calculate heights accurately
        setTimeout(() => {
            // Check if the actual scroll height is greater than the visible client height
            // Add a small buffer (e.g., 1 or 2 pixels) for tolerance
            const isOverflowing = contentContainer.scrollHeight > contentContainer.clientHeight + 1;

            if (isOverflowing) {
                // console.log(`Item ${item.id} is overflowing. Showing View More.`); // Optional debug
                viewMoreLink.style.display = 'inline'; // Show the link
                // Ensure listener is attached only once or remove previous if re-running
                viewMoreLink.removeEventListener('click', handleViewMoreClick); // Remove first
                viewMoreLink.addEventListener('click', handleViewMoreClick);   // Add again
            } else {
                // console.log(`Item ${item.id} is NOT overflowing. Hiding View More.`); // Optional debug
                viewMoreLink.style.display = 'none'; // Hide if not overflowing
                viewMoreLink.removeEventListener('click', handleViewMoreClick); // Remove listener if not needed
            }
        }, 50); // Delay allows browser rendering time
    });
}

// Function to handle clicking the 'View More' link
function handleViewMoreClick(event) {
    event.preventDefault(); // Prevent default anchor behavior (like navigating to '#')
    const link = event.currentTarget;
    const modal = document.getElementById('mood-detail-modal');

    // console.log("View More clicked, dataset:", link.dataset); // Optional debug

    if (!modal) {
        console.error("Mood detail modal element not found!");
        return;
    }

    // --- Get data from the clicked link's data attributes ---
    const title = link.dataset.title || 'Mood Entry';
    const emotion = link.dataset.emotion || 'Unknown';
    const emoji = link.dataset.emoji || '❓';
    const createdAt = link.dataset.createdAt || 'N/A';
    const fullContent = link.dataset.fullContent || '<p>Content not available.</p>'; // Use raw HTML

    // --- ADDED DEBUGGING LOG ---
    console.log("Retrieved fullContent for modal:", fullContent);
    // --- END DEBUGGING LOG ---

    // --- Populate the modal ---
    const modalTitleEl = modal.querySelector('#modal-title');
    const modalEmojiEl = modal.querySelector('#modal-emoji');
    const modalEmotionNameEl = modal.querySelector('#modal-emotion-name');
    const modalCreatedAtEl = modal.querySelector('#modal-created-at');
    const modalFullContentEl = modal.querySelector('#modal-full-content');

    // Check if elements exist before setting content
    if (modalTitleEl) modalTitleEl.textContent = title;
    else console.error("Modal title element not found");

    if (modalEmojiEl) modalEmojiEl.textContent = emoji;
    else console.error("Modal emoji element not found");

    if (modalEmotionNameEl) modalEmotionNameEl.textContent = emotion;
    else console.error("Modal emotion name element not found");

    if (modalCreatedAtEl) modalCreatedAtEl.textContent = createdAt;
    else console.error("Modal created at element not found");

    if (modalFullContentEl) {
        modalFullContentEl.innerHTML = fullContent; // Use innerHTML to render the stored HTML
    } else {
        console.error("Modal full content element not found");
    }

    // --- Show the modal ---
    modal.classList.add('is-visible');
    document.body.style.overflow = 'hidden'; // Prevent background scrolling
    // console.log("Modal should be visible now."); // Optional debug
}

// Function to set up modal close functionality
function setupModalClose() {
    const modal = document.getElementById('mood-detail-modal');
    // Ensure modal exists before trying to query inside it
    if (!modal) {
        // This might happen if the dashboard initially loads with no entries/modal structure
        // console.log("Mood detail modal element not found on initial setup.");
        return;
    }

    const closeButton = modal.querySelector('#modal-detail-close-button');

    if (!closeButton) {
        console.error('Modal close button not found!');
        return;
    }

    const closeModal = () => {
        // Double check modal exists when closing
        const currentModal = document.getElementById('mood-detail-modal');
        if (currentModal && currentModal.classList.contains('is-visible')) {
             currentModal.classList.remove('is-visible');
             document.body.style.overflow = ''; // Restore background scrolling
             // console.log("Modal closed."); // Optional debug
        }
    };

    // Add listeners only once using dataset flags
    if (!closeButton.dataset.listenerAttached) {
         closeButton.addEventListener('click', closeModal);
         closeButton.dataset.listenerAttached = 'true'; // Mark as attached
    }

    // Close modal if clicking outside the content area (on the overlay)
     if (!modal.dataset.overlayListenerAttached) {
        modal.addEventListener('click', (event) => {
            // Check if the click was directly on the modal overlay (not its children)
            if (event.target === modal) {
                closeModal();
            }
        });
         modal.dataset.overlayListenerAttached = 'true'; // Mark as attached
     }

    // Close modal with Escape key - Attach listener to document once
     if (!document.body.dataset.escapeListenerAttached) {
        document.addEventListener('keydown', (event) => {
            const currentModal = document.getElementById('mood-detail-modal'); // Check existence inside handler
            if (event.key === 'Escape' && currentModal && currentModal.classList.contains('is-visible')) {
                closeModal();
            }
        });
         document.body.dataset.escapeListenerAttached = 'true'; // Mark as attached
     }
}

// --- Initial Setup on Page Load ---
// Waits for the entire HTML document to be loaded and parsed.
document.addEventListener('DOMContentLoaded', () => {
    console.log("DOM fully loaded and parsed."); // Debug log
    checkAndSetupViewMore(); // Check existing items on load
    setupModalClose();      // Set up close listeners once
});

// --- Setup after HTMX Content Swaps ---
// Listens for HTMX finishing a swap operation anywhere on the body.
document.body.addEventListener('htmx:afterSwap', function(event) {
    // Check if the swapped content (event.detail.target) is relevant
    // e.g., if it's the dashboard content area or contains mood items.
    const targetArea = document.getElementById('dashboard-content-area'); // Or use a class selector
    if (targetArea && (targetArea.contains(event.detail.target) || event.detail.target === targetArea || event.detail.target.querySelector('.mood-item'))) {
        console.log('htmx:afterSwap detected - Re-running View More setup.'); // Debug log
        checkAndSetupViewMore(); // Re-check for overflow on newly added/swapped items
        // No need to re-run setupModalClose generally, unless the modal itself is swapped.
    }
});