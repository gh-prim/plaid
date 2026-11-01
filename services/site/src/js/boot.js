// Runs before first paint: marks the page as scripted so the reveal
// animations may hide content until it scrolls into view. Without this
// file, or without JavaScript, everything is simply visible.
document.documentElement.classList.add('js');
