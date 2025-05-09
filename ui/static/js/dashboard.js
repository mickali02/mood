document.addEventListener('DOMContentLoaded', function () {
    console.log("Dashboard JS Loaded - View More always visible mode.");

    // --- Mood Detail Modal Logic ---
    const moodDetailModal = document.getElementById('mood-detail-modal');
    const modalDetailCloseButton = document.getElementById('modal-detail-close-button');
    const modalTitle = document.getElementById('modal-title');
    const modalEmoji = document.getElementById('modal-emoji');
    const modalEmotionName = document.getElementById('modal-emotion-name');
    const modalCreatedAt = document.getElementById('modal-created-at');
    const modalFullContent = document.getElementById('modal-full-content');

    function updateModalContent(data) {
        // ... (function remains the same - make sure it handles potential errors gracefully) ...
        if (modalTitle) modalTitle.textContent = data.title || '';
        if (modalEmoji) {
            modalEmoji.textContent = data.emoji || '❓';
            modalEmoji.style.color = data.color || '#ccc';
        }
        if (modalEmotionName) modalEmotionName.textContent = data.emotion || '';
        if (modalCreatedAt) modalCreatedAt.textContent = data.createdAt || '';
        if (modalFullContent) {
             let fullContentHTML = '';
             try {
                 let rawContent = data.fullContent || '';
                 if (rawContent.startsWith('"') && rawContent.endsWith('"')) {
                    rawContent = rawContent.substring(1, rawContent.length - 1);
                 }
                 fullContentHTML = rawContent.replace(/\\"/g, '"').replace(/\\'/g, "'").replace(/\\\\/g, "\\");
             } catch (e) {
                 console.error("Error processing mood content:", e, " Raw data:", data.fullContent);
                 fullContentHTML = "<p>Error displaying content.</p>";
             }
             modalFullContent.innerHTML = fullContentHTML;
        }
    }

    // --- Event Listener for View More Links (DELEGATION ON BODY) ---
    // **** Attach listener to document.body ****
    console.log("[Initial Load] Attaching view more listener to document.body"); // DEBUG
    document.body.addEventListener('click', function(event) {
        // **** Use more specific selector including the container ID ****
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

    // --- REMOVED: Overflow Check Logic ---
    // --- REMOVED: HTMX Event Listener for overflow check ---

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