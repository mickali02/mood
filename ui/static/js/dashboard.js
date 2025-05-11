// Wait for the entire HTML document to be fully loaded and parsed.
document.addEventListener('DOMContentLoaded', function () {
    // Log to confirm the dashboard JavaScript file has been loaded and executed.
    console.log("Dashboard JS Loaded - View More always visible mode.");

    // --- Mood Detail Modal Logic ---
    // Get references to all necessary HTML elements for the mood detail modal.
    const moodDetailModal = document.getElementById('mood-detail-modal');                 // The modal container itself. 
    const modalDetailCloseButton = document.getElementById('modal-detail-close-button');  // The 'x' button to close the modal.
    const modalTitle = document.getElementById('modal-title');                            // Element to display the mood's title.
    const modalEmoji = document.getElementById('modal-emoji');                             // Element for the mood's emoji.
    const modalEmotionName = document.getElementById('modal-emotion-name');                // Element for the mood's emotion name.
    const modalCreatedAt = document.getElementById('modal-created-at');                    // Element for the mood's creation date
    const modalFullContent = document.getElementById('modal-full-content');                 // Element to display the full mood content.


     // Function to populate the modal with data from a mood entry.
    // This function dynamically updates the modal's content based on the clicked mood entry.
    function updateModalContent(data) {
         // Safely set text content for each modal element, providing defaults if data is missing.
        if (modalTitle) modalTitle.textContent = data.title || ''; // Default if title is undefined.
        if (modalEmoji) {
            modalEmoji.textContent = data.emoji || '❓'; // Default emoji.
            modalEmoji.style.color = data.color || '#ccc'; // Default color.
        }
        if (modalEmotionName) modalEmotionName.textContent = data.emotion || '';
        if (modalCreatedAt) modalCreatedAt.textContent = data.createdAt || '';
        if (modalFullContent) {
            // Process and sanitize the raw HTML content for display.
            // This handles escaped quotes that might come from `printf "%q"` in Go templates.
             let fullContentHTML = '';
             try {
                 let rawContent = data.fullContent || ''; // Get raw content, default to empty.
                   // Unescape if content was quoted (e.g., by Go's %q formatting).
                 if (rawContent.startsWith('"') && rawContent.endsWith('"')) {
                    rawContent = rawContent.substring(1, rawContent.length - 1);
                 }
                 // Replace escaped quotes and backslashes.
                 fullContentHTML = rawContent.replace(/\\"/g, '"').replace(/\\'/g, "'").replace(/\\\\/g, "\\");
             } catch (e) {
                 console.error("Error processing mood content:", e, " Raw data:", data.fullContent);
                 fullContentHTML = "<p>Error displaying content.</p>";
             }
             modalFullContent.innerHTML = fullContentHTML;
        }
    }

    // --- Event Listener for View More Links (Event Delegation on document.body) ---
    // Using event delegation for 'View More' links. This means one listener on the body
    // efficiently handles clicks on any current or future 'View More' links, especially important with HTMX content swaps.
    console.log("[Initial Load] Attaching view more listener to document.body"); // DEBUG
    document.body.addEventListener('click', function(event) {
         // Check if the clicked element (or its ancestor) is a 'view-more-link' INSIDE the dashboard content area.
        // `event.target.closest()` efficiently finds the nearest ancestor matching the selector.
        const link = event.target.closest('#dashboard-content-area .view-more-link');

        // console.log("[Body Click] Target:", event.target); // Optional broader debug
        // console.log("[Body Click] Closest link inside content area:", link); // Optional broader debug

        if (link) {
            event.preventDefault();
            const moodData = link.dataset;
            console.log("[Body Click] View More link found! Opening modal with data:", moodData); // DEBUG

            // Add checks for modal elements *just in case*
            if (!moodDetailModal || !modalDetailCloseButton) {
                console.error("ERROR: Modal element(s) not found when trying to open!");
                return;
            }

            try {
                updateModalContent(moodData);
                moodDetailModal.style.display = 'flex';
                setTimeout(() => {
                    if (moodDetailModal) { // Check again inside timeout
                         moodDetailModal.classList.add('is-visible');
                         console.log("[Body Click] Added is-visible class."); // DEBUG
                    }
                }, 10);
            } catch (updateError) {
                 console.error("[Body Click] Error occurred during modal update/show:", updateError); // DEBUG
            }
        }
    });

    // Modal closing logic (Needs checks as modal variable is outside direct scope)
    if (modalDetailCloseButton && moodDetailModal) {
        modalDetailCloseButton.addEventListener('click', function () {
            moodDetailModal.classList.remove('is-visible');
            setTimeout(() => moodDetailModal.style.display = 'none', 300);
        });
    } else {
         if (!modalDetailCloseButton) console.warn("#modal-detail-close-button not found.");
    }

    if (moodDetailModal) {
        moodDetailModal.addEventListener('click', function (event) {
            if (event.target === moodDetailModal) { // Click on backdrop
                moodDetailModal.classList.remove('is-visible');
                setTimeout(() => moodDetailModal.style.display = 'none', 300);
            }
        });
    } else {
         if (!moodDetailModal) console.warn("#mood-detail-modal not found.");
    }
    // --- End Event Listener ---


    // --- Flash Message Close Button Logic (Delegation) ---
    document.body.addEventListener('click', function(event) {
        if (event.target.classList.contains('flash-close-btn')) {
            const flash = event.target.closest('.flash-message');
            if (flash) {
                flash.style.opacity = '0';
                setTimeout(function() {
                    flash.style.display = 'none';
                    flash.remove();
                }, 300);
            }
        }
    });

    // Close modals with ESC key
    document.addEventListener('keydown', function (event) {
        if (event.key === "Escape") {
            // Check modal variable directly
            if (moodDetailModal && (moodDetailModal.style.display === 'flex' || moodDetailModal.classList.contains('is-visible'))) {
                moodDetailModal.classList.remove('is-visible');
                setTimeout(() => {
                    if (moodDetailModal) moodDetailModal.style.display = 'none';
                }, 300);
            }
        }
    });

}); // End DOMContentLoaded