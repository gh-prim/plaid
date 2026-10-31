// Runs before first paint: applies the light theme if the visitor chose it.
// Dark is the default. The choice stays in this browser and is never sent anywhere.
(function () {
  try {
    if (localStorage.getItem('plaid-theme') === 'light') {
      document.documentElement.setAttribute('data-theme', 'light');
    }
  } catch (e) {
    /* storage blocked: keep the default */
  }
})();
