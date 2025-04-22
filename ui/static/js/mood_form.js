// mood/ui/static/js/mood_form.js

document.addEventListener('DOMContentLoaded', () => {
    console.log("Mood Form JS Loaded v4 - Modal Focus");

    // --- Element Selection ---
    const emotionOptionsContainer = document.getElementById('emotion-options-container');
    const finalEmotionNameInput = document.getElementById('final_emotion_name');
    const finalEmotionEmojiInput = document.getElementById('final_emotion_emoji');
    const finalEmotionColorInput = document.getElementById('final_emotion_color');
    const modal = document.getElementById('custom-emotion-modal');
    const modalCloseButton = document.getElementById('modal-close-button');
    const modalCancelButton = document.getElementById('modal-cancel-button');
    const modalSaveButton = document.getElementById('modal-save-button');
    const customEmotionNameInput = document.getElementById('custom_emotion_name');
    const customEmotionEmojiInput = document.getElementById('custom_emotion_emoji');
    const customEmotionColorInput = document.getElementById('custom_emotion_color');
    const emojiSuggestionButtons = document.querySelectorAll('.emoji-suggestion-btn');
    const customNameError = document.getElementById('custom-name-error');
    const customEmojiError = document.getElementById('custom-emoji-error');
    const customColorError = document.getElementById('custom-color-error');
    const moodForm = document.getElementById('mood-entry-form');
    const otherOptionRadio = document.getElementById('emotion-other');
    const otherOptionLabel = document.querySelector('label[for="emotion-other"]');
    const otherOptionEmojiSpan = otherOptionLabel?.querySelector('.emotion-option-emoji');
    const otherOptionNameSpan = otherOptionLabel?.querySelector('.emotion-option-name');

    // --- Helper Functions ---
    function showModal() {
        if (!modal) { console.error("Modal element not found!"); return; }
        console.log(">>> showModal called"); // Focus log
        modal.style.display = 'flex';
        setTimeout(() => modal.classList.add('is-visible'), 10);
        clearModalErrors();

        if (otherOptionRadio?.hasAttribute('data-custom-name')) {
            console.log("   Pre-filling modal with stored custom data.");
            customEmotionNameInput.value = otherOptionRadio.dataset.customName || '';
            customEmotionEmojiInput.value = otherOptionRadio.dataset.customEmoji || '';
            customEmotionColorInput.value = otherOptionRadio.dataset.customColor || '#cccccc';
        } else {
            console.log("   Resetting modal fields on open.");
            resetModalFields();
        }
    }

    function closeModal(isCancelAction = false) {
        if (!modal) return;
        console.log(`>>> closeModal called (isCancelAction: ${isCancelAction})`);
        modal.classList.remove('is-visible');
        setTimeout(() => { modal.style.display = 'none'; }, 300);
        clearModalErrors();

        if (isCancelAction) {
             console.log("   Handling cancel action...");
            const previouslyChecked = document.querySelector('input[name="emotion_choice"].was-checked');
            if (previouslyChecked) {
                console.log("   Re-selecting previous:", previouslyChecked.value);
                previouslyChecked.checked = true;
                updateFinalFields(previouslyChecked);
                resetOtherButtonAppearance();
            } else {
                console.log("   Unchecking 'Other', no previous selection.");
                if(otherOptionRadio) otherOptionRadio.checked = false;
                if (!otherOptionRadio?.hasAttribute('data-custom-name')) { clearFinalFields(); }
                resetOtherButtonAppearance();
            }
             document.querySelectorAll('input[name="emotion_choice"].was-checked').forEach(el => el.classList.remove('was-checked'));
        } else {
             console.log("   Closing after save, no changes to selection/fields needed here.");
        }
    }

    function resetModalFields() { /* ... unchanged ... */
         if(customEmotionNameInput) customEmotionNameInput.value = '';
         if(customEmotionEmojiInput) customEmotionEmojiInput.value = '';
         if(customEmotionColorInput) customEmotionColorInput.value = '#cccccc';
    }
    function clearModalErrors() { /* ... unchanged ... */
        if(customNameError) customNameError.textContent = ''; if(customEmojiError) customEmojiError.textContent = ''; if(customColorError) customColorError.textContent = '';
        if(customEmotionNameInput) customEmotionNameInput.classList.remove('invalid'); if(customEmotionEmojiInput) customEmotionEmojiInput.classList.remove('invalid'); if(customEmotionColorInput) customEmotionColorInput.classList.remove('invalid');
    }
    function validateModal() { /* ... unchanged ... */
        console.log("Validating modal..."); clearModalErrors(); let isValid = true;
        const name = customEmotionNameInput?.value?.trim() || ''; if (!name) { console.log("Modal validation failed: Name missing."); if(customNameError) customNameError.textContent = 'Name must be provided.'; customEmotionNameInput?.classList.add('invalid'); isValid = false; } else if (name.length > 50) { console.log("Modal validation failed: Name too long."); if(customNameError) customNameError.textContent = 'Name must not be more than 50 characters.'; customEmotionNameInput?.classList.add('invalid'); isValid = false; } const emoji = customEmotionEmojiInput?.value?.trim() || ''; if (!emoji) { console.log("Modal validation failed: Emoji missing."); if(customEmojiError) customEmojiError.textContent = 'Emoji must be provided.'; customEmotionEmojiInput?.classList.add('invalid'); isValid = false; } else if (emoji.length === 0 || emoji.length > 5) { console.log("Modal validation failed: Emoji length invalid."); if(customEmojiError) customEmojiError.textContent = 'Please enter a valid emoji (1-5 characters).'; customEmotionEmojiInput?.classList.add('invalid'); isValid = false; } const color = customEmotionColorInput?.value || ''; const hexColorRegex = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/i; if (!hexColorRegex.test(color)) { console.log("Modal validation failed: Color format invalid."); if(customColorError) customColorError.textContent = 'Invalid color format (#RRGGBB or #RGB).'; customEmotionColorInput?.classList.add('invalid'); isValid = false; } console.log("Modal validation result:", isValid); return isValid;
     }
    function updateFinalFields(selectedRadio) { /* ... unchanged ... */
        if (!finalEmotionNameInput || !finalEmotionEmojiInput || !finalEmotionColorInput) { console.error("Cannot update final fields: one or more hidden inputs not found."); return; } if (selectedRadio && selectedRadio.id === 'emotion-other') { if (selectedRadio.hasAttribute('data-custom-name')) { console.log("Updating final fields from 'Other' radio's custom data."); finalEmotionNameInput.value = selectedRadio.dataset.customName; finalEmotionEmojiInput.value = selectedRadio.dataset.customEmoji; finalEmotionColorInput.value = selectedRadio.dataset.customColor; } else { console.log("Updating final fields: 'Other' selected but no custom data found, clearing."); clearFinalFields(); } } else if (selectedRadio) { console.log("Updating final fields from default radio:", selectedRadio.value); finalEmotionNameInput.value = selectedRadio.value; finalEmotionEmojiInput.value = selectedRadio.dataset.emoji || '❓'; finalEmotionColorInput.value = selectedRadio.dataset.color || '#cccccc'; } else { console.log("No radio selected, clearing final fields."); clearFinalFields(); } console.log(" --> Final Fields Set To:", finalEmotionNameInput.value, finalEmotionEmojiInput.value, finalEmotionColorInput.value);
     }
    function clearFinalFields() { /* ... unchanged ... */
        if (!finalEmotionNameInput || !finalEmotionEmojiInput || !finalEmotionColorInput) return; console.log("Clearing final fields."); finalEmotionNameInput.value = ''; finalEmotionEmojiInput.value = ''; finalEmotionColorInput.value = '';
     }
    function getSelectedEmotionChoice() { /* ... unchanged ... */
         const checkedRadio = document.querySelector('input[name="emotion_choice"]:checked'); return checkedRadio ? checkedRadio.value : null;
     }
     function resetOtherButtonAppearance() { /* ... unchanged ... */
        if (otherOptionLabel && otherOptionEmojiSpan && otherOptionNameSpan && otherOptionRadio) { console.log("Resetting 'Other...' button appearance and data."); otherOptionEmojiSpan.textContent = '➕'; otherOptionNameSpan.textContent = 'Other...'; otherOptionLabel.style.borderColor = ''; otherOptionLabel.style.backgroundColor = ''; otherOptionLabel.classList.remove('has-custom-value'); otherOptionRadio.removeAttribute('data-custom-name'); otherOptionRadio.removeAttribute('data-custom-emoji'); otherOptionRadio.removeAttribute('data-custom-color'); }
     }
     function updateOtherButtonAppearance(name, emoji, color) { /* ... unchanged ... */
          if (otherOptionLabel && otherOptionEmojiSpan && otherOptionNameSpan) { console.log("Updating 'Other...' button appearance."); otherOptionEmojiSpan.textContent = emoji || '❓'; otherOptionNameSpan.textContent = name; otherOptionLabel.style.borderColor = color; otherOptionLabel.classList.add('has-custom-value'); } else { console.warn("'Other...' label or its spans not found. Cannot update appearance."); }
     }

    // --- Event Listeners ---

    if (emotionOptionsContainer) {
        // *** SIMPLIFIED/CORRECTED Click Listener ***
        emotionOptionsContainer.addEventListener('click', (event) => {
            const label = event.target.closest('label.emotion-option');
            if (!label) return; // Ignore clicks not on a label

            const radioId = label.getAttribute('for');
            const radio = document.getElementById(radioId);

            if (radio && radio.name === 'emotion_choice') {
                console.log(`Click detected on label for: ${radioId}`);

                // Get the currently checked radio *before* this click potentially changes it
                 const currentlyCheckedRadio = document.querySelector('input[name="emotion_choice"]:checked');

                 // Mark the currently checked one IF it's a default one and different from the clicked one
                 if (currentlyCheckedRadio && currentlyCheckedRadio !== radio && currentlyCheckedRadio.id !== 'emotion-other') {
                      console.log("Marking previously checked:", currentlyCheckedRadio.id);
                      document.querySelectorAll('input[name="emotion_choice"].was-checked').forEach(el => el.classList.remove('was-checked')); // Clear previous marks
                      currentlyCheckedRadio.classList.add('was-checked');
                 }


                if (radio.id === 'emotion-other') {
                    // Clicked on "Other..." label
                    console.log("'Other...' label clicked.");
                    // Check the radio (might already be checked if re-clicked)
                    if(!radio.checked) radio.checked = true;
                    // Always open the modal when "Other..." is clicked
                    showModal();
                     // If it already has custom data, update hidden fields now
                     if (otherOptionRadio?.hasAttribute('data-custom-name')) {
                         updateFinalFields(otherOptionRadio);
                     }

                } else {
                    // Clicked on a DEFAULT emotion label
                    console.log("Default emotion label clicked:", radio.value);
                    if (!radio.checked) {
                        radio.checked = true; // Ensure it gets checked
                        updateFinalFields(radio); // Update hidden fields
                        resetOtherButtonAppearance(); // Reset the "Other..." button visuals
                    } else {
                        // Clicking already selected default - do nothing extra
                        console.log("Re-clicked already selected default.");
                    }
                }
            }
        });
        // --- End Simplified Listener ---

    } else {
         console.error("CRITICAL: Emotion options container ('#emotion-options-container') not found.");
    }

    // Modal close/cancel buttons - pass true
    if (modalCloseButton) modalCloseButton.addEventListener('click', () => closeModal(true));
    if (modalCancelButton) modalCancelButton.addEventListener('click', () => closeModal(true));

    // Save custom emotion from modal
    if (modalSaveButton) { /* ... same save listener as previous correct version ... */
         modalSaveButton.addEventListener('click', () => { console.log("[Save Custom Clicked]"); if (validateModal()) { console.log("  Modal Validated."); if (finalEmotionNameInput && finalEmotionEmojiInput && finalEmotionColorInput && otherOptionRadio) { const nameVal = customEmotionNameInput.value.trim(); const emojiVal = customEmotionEmojiInput.value.trim(); const colorVal = customEmotionColorInput.value; console.log(`  Values from modal: Name='${nameVal}', Emoji='${emojiVal}', Color='${colorVal}'`); finalEmotionNameInput.value = nameVal; finalEmotionEmojiInput.value = emojiVal; finalEmotionColorInput.value = colorVal; console.log(`  Values in hidden inputs AFTER assignment: Name='${finalEmotionNameInput.value}', Emoji='${finalEmotionEmojiInput.value}', Color='${finalEmotionColorInput.value}'`); otherOptionRadio.dataset.customName = nameVal; otherOptionRadio.dataset.customEmoji = emojiVal; otherOptionRadio.dataset.customColor = colorVal; console.log("  Stored custom data on 'Other' radio attributes."); updateOtherButtonAppearance(nameVal, emojiVal, colorVal); otherOptionRadio.checked = true; console.log("  Ensured 'Other' radio is checked."); } else { console.error("  ERROR: Crucial elements missing during save!"); } document.querySelectorAll('input[name="emotion_choice"].was-checked').forEach(el => el.classList.remove('was-checked')); console.log("  Closing modal after save (not a cancel action)."); closeModal(false); } else { console.log("  Modal validation failed. Not saving or closing."); } });
     } else { console.error("Modal save button not found!"); }

    // Emoji Suggestions Listener
    if (emojiSuggestionButtons) { /* ... same listener as previous correct version ... */
         console.log("Adding emoji suggestion button listeners..."); emojiSuggestionButtons.forEach(button => { button.addEventListener('click', () => { if(customEmotionEmojiInput) { customEmotionEmojiInput.value = button.textContent; console.log("Emoji suggestion clicked, input set to:", button.textContent); } else { console.error("Custom emoji input field not found!"); } if(customEmojiError) { customEmojiError.textContent = ''; } if(customEmotionEmojiInput) { customEmotionEmojiInput.classList.remove('invalid'); } }); });
     } else { console.warn("Emoji suggestion buttons not found. Listeners not added."); }

    // Backdrop Click Listener
    if (modal) { modal.addEventListener('click', (event) => { if (event.target === modal) { console.log("Clicked modal backdrop (cancel action)."); closeModal(true); } }); } else { console.error("Modal element not found for backdrop click listener!"); }

     // --- Initial State Logic ---
     function initializeFormState() { /* ... same initialization logic as previous correct version ... */
         if (!finalEmotionNameInput || !finalEmotionEmojiInput || !finalEmotionColorInput || !otherOptionRadio) { console.warn("Mood form essential fields not found, skipping initialization."); return; } console.log("Initializing form state..."); const initialHiddenName = finalEmotionNameInput.value; const initialHiddenEmoji = finalEmotionEmojiInput.value; const initialHiddenColor = finalEmotionColorInput.value; let initiallySelectedRadio = document.querySelector('input[name="emotion_choice"]:checked'); let isInitialCustom = false; if (initialHiddenName) { const defaultRadios = document.querySelectorAll('.default-emotion-radio'); isInitialCustom = !Array.from(defaultRadios).some(radio => radio.value === initialHiddenName); } console.log("Initial hidden name:", initialHiddenName || "''"); console.log("Initially selected radio:", initiallySelectedRadio ? initiallySelectedRadio.value : 'None'); console.log("Is initial state custom:", isInitialCustom); if (initiallySelectedRadio) { if (initiallySelectedRadio.value === 'other') { if (isInitialCustom) { console.log("Initializing: 'Other' radio checked, is custom. Setting appearance and data attributes."); otherOptionRadio.dataset.customName = initialHiddenName; otherOptionRadio.dataset.customEmoji = initialHiddenEmoji; otherOptionRadio.dataset.customColor = initialHiddenColor; updateOtherButtonAppearance(initialHiddenName, initialHiddenEmoji, initialHiddenColor); } else { console.log("Initializing: 'Other' radio checked, but no custom data found? Clearing fields/resetting."); clearFinalFields(); resetOtherButtonAppearance(); } } else { console.log("Initializing: Default radio checked:", initiallySelectedRadio.value); updateFinalFields(initiallySelectedRadio); resetOtherButtonAppearance(); } } else if (isInitialCustom) { console.log("Initializing: No radio checked, detected custom emotion. Checking 'Other' and setting state."); otherOptionRadio.checked = true; otherOptionRadio.dataset.customName = initialHiddenName; otherOptionRadio.dataset.customEmoji = initialHiddenEmoji; otherOptionRadio.dataset.customColor = initialHiddenColor; updateOtherButtonAppearance(initialHiddenName, initialHiddenEmoji, initialHiddenColor); updateFinalFields(otherOptionRadio); initiallySelectedRadio = otherOptionRadio; } else if (initialHiddenName) { console.log("Initializing: No radio checked, detected default emotion. Checking its radio."); const matchingRadio = document.getElementById(`emotion-${initialHiddenName}`); if (matchingRadio) { matchingRadio.checked = true; updateFinalFields(matchingRadio); initiallySelectedRadio = matchingRadio;} resetOtherButtonAppearance(); } else { console.log("Initializing: Fresh form. Clearing fields and resetting 'Other' button."); clearFinalFields(); resetOtherButtonAppearance(); if(initiallySelectedRadio) initiallySelectedRadio.checked = false; }
     }

     // Run initialization logic
     initializeFormState();

}); // End DOMContentLoaded