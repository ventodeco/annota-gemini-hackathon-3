function captureSelection() {
    const textElement = document.getElementById('ocr-text');
    const selection = window.getSelection();
    
    if (selection.rangeCount === 0 || selection.toString().trim() === '') {
        alert('Please select some text first');
        return;
    }
    
    const selectedText = selection.toString().trim();
    const input = document.getElementById('selected-text-input');
    
    if (input) {
        input.value = selectedText;
        
        const form = input.closest('form');
        if (form) {
            htmx.trigger(form, 'submit');
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const textElement = document.getElementById('ocr-text');
    if (textElement) {
        textElement.addEventListener('mouseup', () => {
            const selection = window.getSelection();
            if (selection.toString().trim() !== '') {
                console.log('Text selected:', selection.toString());
            }
        });
    }
});
