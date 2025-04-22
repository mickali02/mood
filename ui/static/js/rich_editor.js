// mood/ui/static/js/rich_editor.js

document.addEventListener('DOMContentLoaded', () => {
    const editorContainer = document.getElementById('editor-container');
    const hiddenInput = document.getElementById('content'); // Our hidden input

    if (editorContainer && hiddenInput) {
        // Define the toolbar options
        // Adding 'background' enables the highlighting feature
        const toolbarOptions = [
            [{ 'header': [1, 2, 3, false] }], // Headers
            ['bold', 'italic', 'underline'],   // Basic formatting
            [{ 'list': 'ordered'}, { 'list': 'bullet' }], // Lists
            [{ 'background': [] }],            // Highlighting (background color)
            ['clean']                          // Remove formatting button
        ];

        // Initialize Quill
        const quill = new Quill(editorContainer, {
            modules: {
                toolbar: toolbarOptions
            },
            theme: 'snow', // Use the 'snow' theme
            placeholder: 'Describe how you\'re feeling...' // Set placeholder text
        });

        // --- Pre-populate editor if editing (handles potential future use) ---
        // If the hidden input already has HTML content (e.g., from server-side
        // rendering on an edit form), load it into the Quill editor.
        const initialHtml = hiddenInput.value;
        if (initialHtml) {
            // Use clipboard.convert to safely load HTML
            // It might not perfectly preserve all styles on load, but it's safer.
            const delta = quill.clipboard.convert(initialHtml);
            quill.setContents(delta, 'silent'); // 'silent' prevents firing text-change initially
             console.log("Loaded initial content into Quill:", initialHtml); // Debugging
        } else {
             console.log("Quill initialized empty."); // Debugging
        }


        // --- Sync Quill content to hidden input on change ---
        quill.on('text-change', (delta, oldDelta, source) => {
            // Get the HTML content from Quill's root element
            const htmlContent = quill.root.innerHTML;
            // Update the hidden input's value
            hiddenInput.value = htmlContent;
             // console.log("Quill content changed, hidden input updated:", htmlContent); // Debugging
        });

        // --- Optional: Ensure initial state is synced if pre-populated ---
        // This handles cases where the initial content might not trigger text-change
        // (though setContents usually does). It's a safety net.
        // We run this slightly delayed to ensure everything is settled.
        setTimeout(() => {
            if (hiddenInput.value === '' && quill.getLength() > 1) { // Check if quill has content but hidden is empty
                hiddenInput.value = quill.root.innerHTML;
                 console.log("Forcing initial sync to hidden input."); // Debugging
            }
        }, 100);


    } else {
        console.error("Quill editor container or hidden input not found!");
    }
});