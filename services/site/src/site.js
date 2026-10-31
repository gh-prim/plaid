// Progressive enhancement only: the site works without this file.
// Labels come from data attributes rendered from the locale files.
(function () {
  var root = document.documentElement;

  // Theme toggle (dark by default).
  var toggle = document.querySelector('.theme-toggle');
  if (toggle) {
    var sync = function () {
      toggle.setAttribute('aria-pressed', String(root.getAttribute('data-theme') === 'light'));
    };
    toggle.hidden = false;
    sync();
    toggle.addEventListener('click', function () {
      var light = root.getAttribute('data-theme') !== 'light';
      if (light) root.setAttribute('data-theme', 'light');
      else root.removeAttribute('data-theme');
      try {
        if (light) localStorage.setItem('plaid-theme', 'light');
        else localStorage.removeItem('plaid-theme');
      } catch (e) {
        /* storage blocked: the choice lasts for this page only */
      }
      sync();
    });
  }

  // Copy buttons on code blocks.
  if (!navigator.clipboard) return;
  document.querySelectorAll('figure.code').forEach(function (fig) {
    var button = fig.querySelector('button.copy');
    var code = fig.querySelector('code');
    if (!button || !code) return;
    var label = button.textContent;
    var timer;
    button.hidden = false;
    button.addEventListener('click', function () {
      navigator.clipboard.writeText(code.textContent).then(function () {
        button.textContent = button.getAttribute('data-copied');
        button.classList.add('is-copied');
        clearTimeout(timer);
        timer = setTimeout(function () {
          button.textContent = label;
          button.classList.remove('is-copied');
        }, 2000);
      }, function () {
        // Clipboard refused: select the code so it can be copied by hand.
        var range = document.createRange();
        range.selectNodeContents(code);
        var selection = window.getSelection();
        selection.removeAllRanges();
        selection.addRange(range);
      });
    });
  });
})();
