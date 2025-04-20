// ui/static/js/mood_form.js

document.addEventListener('DOMContentLoaded', () => {
    // --- Get DOM Elements ---
    const form = document.getElementById('mood-entry-form');
    const emotionOptionsContainer = document.getElementById('emotion-options-container');
    const defaultEmotionRadios = document.querySelectorAll('.default-emotion-radio');
    const otherEmotionRadio = document.getElementById('emotion-other');
    const otherOptionLabel = document.querySelector('label[for="emotion-other"]'); // Label for 'Other'

    // Hidden fields that actually get submitted
    const finalEmotionNameInput = document.getElementById('final_emotion_name');
    const finalEmotionEmojiInput = document.getElementById('final_emotion_emoji');
    const finalEmotionColorInput = document.getElementById('final_emotion_color');

    // Modal elements
    const modal = document.getElementById('custom-emotion-modal');
    const modalCloseButton = document.getElementById('modal-close-button');
    const modalCancelButton = document.getElementById('modal-cancel-button');
    const modalSaveButton = document.getElementById('modal-save-button');
    const customNameInput = document.getElementById('custom_emotion_name');
    const customEmojiInput = document.getElementById('custom_emotion_emoji');
    const customColorInput = document.getElementById('custom_emotion_color');

    // Modal error spans
    const customNameError = document.getElementById('custom-name-error');
    const customEmojiError = document.getElementById('custom-emoji-error');
    const customColorError = document.getElementById('custom-color-error'); // If needed for client-side validation

    // --- Get Emoji Suggestion Buttons (Inside Modal) ---
    const emojiSuggestionButtons = document.querySelectorAll('.emoji-suggestion-btn');

    let isCustomEmotionSaved = false; // Flag to track if custom values are set
    let alertShown = false; // Flag to prevent spamming alerts on submit validation

    // --- Helper Functions ---
    function openModal() {
        if (modal) {
            clearModalErrors();
            // Optional: You could pre-fill modal from hidden fields if custom was previously saved
            // if (isCustomEmotionSaved) {
            //     customNameInput.value = finalEmotionNameInput.value;
            //     customEmojiInput.value = finalEmotionEmojiInput.value;
            //     customColorInput.value = finalEmotionColorInput.value;
            // }
            modal.style.display = 'flex'; // Use flex to enable align/justify
        }
    }

    function closeModal() {
        if (modal) {
            modal.style.display = 'none';
            clearModalErrors();
        }
    }

    function clearModalErrors() {
        if(customNameError) customNameError.textContent = '';
        if(customEmojiError) customEmojiError.textContent = '';
        if(customColorError) customColorError.textContent = '';
    }

    function updateHiddenFields(name, emoji, color) {
        finalEmotionNameInput.value = name;
        finalEmotionEmojiInput.value = emoji;
        finalEmotionColorInput.value = color;
        console.log("Hidden fields updated:", { name, emoji, color }); // For debugging
    }

    // Basic validation for modal inputs
    function validateModalInputs() {
        clearModalErrors();
        let isValid = true;
        const name = customNameInput.value.trim();
        const emoji = customEmojiInput.value.trim();
        // Basic Unicode check for something likely an emoji. Doesn't cover all edge cases (like sequences).
        const emojiRegex = /\p{Emoji_Presentation}/u;

        if (name === '') {
            customNameError.textContent = 'Emotion name cannot be blank.';
            isValid = false;
        }
        if (emoji === '') {
             customEmojiError.textContent = 'Emoji cannot be blank.';
             isValid = false;
        // } else if (!emojiRegex.test(emoji)) { // This regex might be too strict, let's rely on backend/user for now
        //      customEmojiError.textContent = 'Please enter a valid emoji character.';
        //      isValid = false;
        } else if (emoji.length > 5) { // Basic length check as fallback
             customEmojiError.textContent = 'Emoji seems too long.';
             isValid = false;
        }
        // Hex color input usually self-validates format

        return isValid;
    }

    // --- Event Listeners ---

    // Add Event Listeners for Emoji Buttons (must be after buttons are selected)
    emojiSuggestionButtons.forEach(button => {
        button.addEventListener('click', () => {
            if (customEmojiInput) {
                customEmojiInput.value = button.textContent; // Set input value to clicked emoji
                if(customEmojiError) customEmojiError.textContent = ''; // Clear error on selection
            }
        });
    });

    // 1. Listen to changes on the radio button group
    if (emotionOptionsContainer) {
        emotionOptionsContainer.addEventListener('change', (event) => {
            const selectedRadio = event.target;

            if (selectedRadio.classList.contains('default-emotion-radio')) {
                // Default emotion selected
                isCustomEmotionSaved = false; // Reset custom flag
                const name = selectedRadio.value;
                const emoji = selectedRadio.dataset.emoji || '❓';
                const color = selectedRadio.dataset.color || '#cccccc';
                updateHiddenFields(name, emoji, color);
                // Reset "Other" label text if it was customized
                if(otherOptionLabel) {
                     otherOptionLabel.querySelector('.emotion-option-emoji').textContent = '➕';
                     otherOptionLabel.querySelector('.emotion-option-name').textContent = 'Other...';
                }
                closeModal(); // Close modal if it was open
            } else if (selectedRadio.id === 'emotion-other') {
                // "Other" selected - open modal
                openModal();
                // Clear hidden fields only if custom wasn't already set and saved
                 if (!isCustomEmotionSaved) {
                    updateHiddenFields('', '', '');
                 }
            }
        });
    }


    // 2. Modal Buttons
    if (modalCloseButton) {
        modalCloseButton.onclick = () => {
             if(otherEmotionRadio && otherEmotionRadio.checked && !isCustomEmotionSaved) {
                otherEmotionRadio.checked = false;
                updateHiddenFields('', '', '');
             }
             closeModal();
        };
    }
    if (modalCancelButton) {
        modalCancelButton.onclick = () => {
             if(otherEmotionRadio && otherEmotionRadio.checked && !isCustomEmotionSaved) {
                otherEmotionRadio.checked = false;
                updateHiddenFields('', '', '');
             }
            closeModal();
        };
    }
    if (modalSaveButton) {
        modalSaveButton.onclick = () => {
            if (validateModalInputs()) {
                const customName = customNameInput.value.trim();
                const customEmoji = customEmojiInput.value.trim();
                const customColor = customColorInput.value;

                updateHiddenFields(customName, customEmoji, customColor);
                isCustomEmotionSaved = true; // Mark custom as saved

                // Update the 'Other' label visually
                 if(otherOptionLabel) {
                     otherOptionLabel.querySelector('.emotion-option-emoji').textContent = customEmoji;
                     otherOptionLabel.querySelector('.emotion-option-name').textContent = customName;
                 }

                closeModal();
            }
        };
    }

    // 3. Prevent form submission if "Other" is selected but modal wasn't saved
    if (form) {
        form.addEventListener('submit', (event) => {
            const checkedRadio = form.querySelector('input[name="emotion_choice"]:checked');

            // Check 1: Is "Other" selected but custom details not saved?
            if (otherEmotionRadio && otherEmotionRadio.checked && !isCustomEmotionSaved) {
                event.preventDefault();
                alert("Please define and save your custom emotion using the 'Other...' option, or select a default emotion.");
                openModal(); // Re-open modal to prompt user
                return; // Stop further checks
            }

            // Check 2: Is any radio button selected at all?
            if (!checkedRadio) {
                event.preventDefault();
                 if (!alertShown) {
                    alert("Please select how you are feeling.");
                    alertShown = true;
                    setTimeout(() => { alertShown = false; }, 100);
                 }
                return;
            }

            // Check 3: Are the final hidden fields populated? (Failsafe)
            if (finalEmotionNameInput.value === '' || finalEmotionEmojiInput.value === '' || finalEmotionColorInput.value === '') {
                 if (!alertShown) {
                    event.preventDefault();
                    alert("An emotion selection is required. Please select a default or define/save a custom one.");
                    alertShown = true;
                    setTimeout(() => { alertShown = false; }, 100);
                 }
                 return;
            }

            // If all checks pass, submission proceeds normally
        });
    }


    // 4. Close modal if clicked outside the content
    window.onclick = (event) => {
        if (modal && event.target === modal) {
             if(otherEmotionRadio && otherEmotionRadio.checked && !isCustomEmotionSaved) {
                otherEmotionRadio.checked = false;
                updateHiddenFields('', '', '');
             }
            closeModal();
        }
    };

    // 5. On page load, check if form data indicates a previous custom selection attempt (e.g., validation error)
    // This helps pre-fill the modal or update the 'Other' label visually if needed.
    const initialFormDataEmotion = finalEmotionNameInput.value; // Check hidden field value passed back from server
    const initialFormDataChoice = form.querySelector('input[name="emotion_choice"]:checked')?.value;

    if(initialFormDataChoice === 'other' || (initialFormDataEmotion && !defaultEmotionRadios.some(radio => radio.value === initialFormDataEmotion))) {
        // If 'other' radio was checked OR if the emotion name doesn't match any default radio value
        if (initialFormDataEmotion) { // If there are custom values from server-side validation failure
            const initialEmoji = finalEmotionEmojiInput.value;
            const initialColor = finalEmotionColorInput.value;

            // Pre-fill modal inputs
            customNameInput.value = initialFormDataEmotion;
            customEmojiInput.value = initialEmoji;
            customColorInput.value = initialColor;

             // Visually update the 'Other' label
            if (otherOptionLabel) {
                otherOptionLabel.querySelector('.emotion-option-emoji').textContent = initialEmoji || '➕';
                otherOptionLabel.querySelector('.emotion-option-name').textContent = initialFormDataEmotion || 'Other...';
            }
             // Mark as saved so submit doesn't fail immediately
            isCustomEmotionSaved = true;
             // Ensure the 'Other' radio button is checked visually
            if(otherEmotionRadio) otherEmotionRadio.checked = true;

        } else if(initialFormDataChoice === 'other') {
             // If 'other' was checked but no custom data came back (e.g., first load failed validation before modal save)
             // Ensure 'Other' radio is checked, but don't mark as saved
              if(otherEmotionRadio) otherEmotionRadio.checked = true;
              isCustomEmotionSaved = false;
        }
    }


}); // End DOMContentLoaded