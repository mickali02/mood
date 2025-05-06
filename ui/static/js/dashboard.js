// mood/ui/static/js/dashboard.js
document.addEventListener('DOMContentLoaded', function () {
    // --- Mood Detail Modal Logic (Keep this) ---
    const moodDetailModal = document.getElementById('mood-detail-modal');
    const modalDetailCloseButton = document.getElementById('modal-detail-close-button');
    const viewMoreLinks = document.querySelectorAll('.view-more-link');

    function setModalTextContent(elementId, text) {
        const element = document.getElementById(elementId);
        if (element) {
            element.textContent = text || '';
        }
    }
    function setModalHTMLContent(elementId, html) {
        const element = document.getElementById(elementId);
        if (element) {
            element.innerHTML = html || '';
        }
    }
    function setModalStyle(elementId, property, value) {
        const element = document.getElementById(elementId);
        if (element) {
            element.style[property] = value || '';
        }
    }

    if (moodDetailModal) {
        viewMoreLinks.forEach(link => {
            link.style.display = 'inline';
            link.addEventListener('click', function (event) {
                event.preventDefault();
                const moodData = this.dataset;
                setModalTextContent('modal-title', moodData.title);
                setModalTextContent('modal-emoji', moodData.emoji);
                setModalStyle('modal-emoji', 'color', moodData.color);
                setModalTextContent('modal-emotion-name', moodData.emotion);
                setModalTextContent('modal-created-at', moodData.createdAt);
                let fullContent = '';
                try {
                    let rawContent = moodData.fullContent;
                    if (rawContent.startsWith('"') && rawContent.endsWith('"')) {
                        rawContent = rawContent.substring(1, rawContent.length - 1);
                    }
                    fullContent = rawContent.replace(/\\"/g, '"').replace(/\\'/g, "'").replace(/\\\\/g, "\\");
                } catch (e) {
                    console.error("Error parsing mood content:", e);
                    fullContent = "<p>Error displaying content.</p>";
                }
                setModalHTMLContent('modal-full-content', fullContent);
                moodDetailModal.style.display = 'flex';
                setTimeout(() => moodDetailModal.classList.add('is-visible'), 10);
            });
        });
        if (modalDetailCloseButton) {
            modalDetailCloseButton.addEventListener('click', function () {
                moodDetailModal.classList.remove('is-visible');
                setTimeout(() => moodDetailModal.style.display = 'none', 300);
            });
        }
        moodDetailModal.addEventListener('click', function (event) {
            if (event.target === moodDetailModal) {
                moodDetailModal.classList.remove('is-visible');
                setTimeout(() => moodDetailModal.style.display = 'none', 300);
            }
        });
    } else {
        // console.warn("Mood detail modal 'mood-detail-modal' not found."); // You can keep or remove this
    }

    // --- Profile Modal Logic (REMOVE THIS ENTIRE SECTION) ---
    // const profileModalTrigger = document.getElementById('profile-modal-trigger');
    // const profileModal = document.getElementById('profile-modal');
    // ... and all related listeners for profileModal ...

    // --- Flash Message Close Button Logic (Keep this) ---
    const flashMessages = document.querySelectorAll('.flash-message');
    flashMessages.forEach(function(flash) {
        const closeButton = flash.querySelector('.flash-close-btn');
        if (closeButton) {
            closeButton.addEventListener('click', function() {
                flash.style.opacity = '0';
                setTimeout(function() {
                    flash.style.display = 'none';
                }, 300);
            });
        }
    });

    // Close modals with ESC key
    document.addEventListener('keydown', function (event) {
        if (event.key === "Escape") {
            // Keep mood detail modal logic
            if (moodDetailModal && (moodDetailModal.style.display === 'flex' || moodDetailModal.classList.contains('is-visible'))) {
                moodDetailModal.classList.remove('is-visible');
                setTimeout(() => moodDetailModal.style.display = 'none', 300);
            }
            // REMOVE profile modal check from here
        }
    });
});