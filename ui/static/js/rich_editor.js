// ui/static/js/rich_editor.js
console.log("rich_editor.js: Script loaded.");

// Wrap initialization logic in a globally accessible function
window.initializeQuill = function() {
    console.log("initializeQuill: Function called.");
    const editorContainer = document.getElementById('editor-container');
    const contentInput = document.getElementById('content'); // Hidden input

    // Exit if elements aren't found (e.g., on pages without the form)
    if (!editorContainer || !contentInput) {
        console.log("initializeQuill: Editor container or hidden input not found. Skipping initialization.");
        return;
    }

    // --- IMPORTANT: Clean up previous Quill instance if it exists ---
    // This prevents errors if HTMX swaps content multiple times.
    // We store the instance on the container element itself.
    if (editorContainer._quillInstance) {
        console.warn("initializeQuill: Previous Quill instance found. Attempting to destroy it.");
        try {
            // Ideally, use Quill's official API if available (check v2 docs)
            // e.g., editorContainer._quillInstance.destroy();
            // If no official destroy method, manually remove the UI elements
            // and hope garbage collection takes care of the JS object.
             editorContainer.innerHTML = ''; // Clear the container's content
             console.log("initializeQuill: Cleared previous editor container HTML.");
             delete editorContainer._quillInstance; // Remove our reference
        } catch (e) {
            console.error("initializeQuill: Error during cleanup of previous Quill instance:", e);
        }
    } else {
         console.log("initializeQuill: No previous Quill instance found on container.");
    }
    // --- End Cleanup ---


    console.log("initializeQuill: Proceeding with new Quill initialization.");
    try {
        // Define the toolbar options
        const toolbarOptions = [
            [{ 'header': [1, 2, 3, false] }],
            ['bold', 'italic', 'underline'],
            [{ 'list': 'ordered'}, { 'list': 'bullet' }],
            [{ 'background': [] }], // Highlighting
            ['clean']
        ];

        // Initialize Quill
        const quill = new Quill(editorContainer, {
            modules: {
                toolbar: toolbarOptions
            },
            theme: 'snow',
            placeholder: 'Describe how you\'re feeling...'
        });

        // Store the new instance on the container element
        editorContainer._quillInstance = quill;
        console.log("initializeQuill: New Quill instance created.");

        // --- Pre-populate editor from hidden input ---
        // This is crucial for repopulating after validation failure
        const initialHtml = contentInput.value;
        if (initialHtml && initialHtml.trim() !== '<p><br></p>' && initialHtml.trim() !== '') {
             console.log("initializeQuill: Found initial HTML in hidden input. Setting editor content.");
             // Use dangerouslyPasteHTML since the hidden input stores raw HTML
             quill.clipboard.dangerouslyPasteHTML(initialHtml);
             console.log("initializeQuill: Content set from hidden input value.");
        } else {
             console.log("initializeQuill: Hidden input is empty or has default placeholder. Editor initialized empty.");
        }

        // --- Sync Quill content back to hidden input on change ---
        quill.on('text-change', (delta, oldDelta, source) => {
            if (source === 'user') {
                let currentContent = quill.root.innerHTML;
                // Don't store the default empty paragraph Quill often uses
                if (currentContent === '<p><br></p>') {
                    currentContent = '';
                }
                contentInput.value = currentContent;
                 // console.log('Quill text-change: Hidden input updated.'); // Keep this commented unless deep debugging
            }
        });

         console.log("initializeQuill: Initialization complete and text-change listener attached.");

    } catch (error) {
        console.error("initializeQuill: CRITICAL ERROR during Quill initialization:", error);
        // Display error in the UI if possible
        if (editorContainer) {
            editorContainer.innerHTML = '<p style="color: red; font-weight: bold;">Error loading text editor. Please try reloading the page.</p>';
        }
    }
}

// --- Event Listener for Initial Page Load ---
document.addEventListener('DOMContentLoaded', () => {
    console.log("DOMContentLoaded: Event fired. Calling initial initializeQuill().");
    window.initializeQuill(); // Run the initializer
});

// --- Event Listener for HTMX Swaps ---
// Listen on the body, as the target element might be replaced entirely.
document.body.addEventListener('htmx:afterSwap', function(event) {
    console.log('htmx:afterSwap: Event detected on body.');

    // Check if the element that HTMX swapped *is* or *contains* the editor container.
    // event.detail.target is the element specified in hx-target
    // event.detail.elt is the element being swapped in (often the same for outerHTML)
    let swappedElement = event.detail.elt;

    // Check if the swapped element itself is the container, or find it within
    const editorInSwapped = (swappedElement.id === 'editor-container')
        ? swappedElement
        : swappedElement.querySelector('#editor-container');

    if (editorInSwapped) {
        console.log("htmx:afterSwap: Editor container #editor-container FOUND in swapped content. Calling initializeQuill().");
        window.initializeQuill(); // Re-run the initializer
    } else {
         console.log("htmx:afterSwap: Editor container #editor-container NOT found directly in swapped content element:", swappedElement.tagName, swappedElement.id);
         // Additional check: sometimes the target might be different from the swapped element
         if (event.detail.target.id === 'editor-container' || event.detail.target.querySelector('#editor-container')) {
             console.log("htmx:afterSwap: Editor container found within the hx-target element. Calling initializeQuill().");
             window.initializeQuill();
         } else {
              console.log("htmx:afterSwap: Editor container also NOT found in hx-target element:", event.detail.target.tagName, event.detail.target.id);
         }
    }
});