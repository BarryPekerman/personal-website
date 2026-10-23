// /terminal: the terminal alone, full screen. Loaded in <head>.

// No terminal on phones (same test as the front page's phone layout): send
// them to the page made for them
if (matchMedia('(max-width: 700px), (max-height: 500px) and (pointer: coarse)').matches) location.replace('/');

// The help popup: the button toggles it, a click anywhere else closes it
document.addEventListener('DOMContentLoaded', () => {
    const popup = document.getElementById('help-popup');
    const content = document.getElementById('help-content');
    popup.addEventListener('click', () => content.classList.toggle('show'));
    document.addEventListener('click', e => {
        if (!popup.contains(e.target) && !content.contains(e.target)) content.classList.remove('show');
    });
});
