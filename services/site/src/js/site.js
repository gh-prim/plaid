// Progressive enhancement only: every page works without this file.
// Labels come from data attributes rendered from the locale files.
(function () {
  var root = document.documentElement;

  // Theme toggle: dark by default, light on request.
  var toggles = document.querySelectorAll('.theme-toggle');
  function syncTheme() {
    var light = root.getAttribute('data-theme') === 'light';
    toggles.forEach(function (b) {
      b.setAttribute('aria-pressed', String(light));
    });
  }
  toggles.forEach(function (b) {
    b.hidden = false;
    b.addEventListener('click', function () {
      var light = root.getAttribute('data-theme') !== 'light';
      root.setAttribute('data-theme', light ? 'light' : 'dark');
      try {
        if (light) localStorage.setItem('plaid-theme', 'light');
        else localStorage.removeItem('plaid-theme');
      } catch (e) {
        /* storage blocked: the choice lasts for this page only */
      }
      syncTheme();
    });
  });
  syncTheme();

  // Copy buttons (the app's CommandBlock): "Copied" for 2 s, announced.
  if (navigator.clipboard) {
    document.querySelectorAll('[data-copy]').forEach(function (button) {
      var block = button.parentNode;
      var code = block.querySelector('code');
      var live = block.querySelector('[aria-live]');
      var timer;
      button.hidden = false;
      button.addEventListener('click', function () {
        navigator.clipboard.writeText(code.textContent).then(
          function () {
            button.textContent = button.getAttribute('data-copied');
            button.classList.add('is-copied');
            live.textContent = button.getAttribute('data-copied');
            clearTimeout(timer);
            timer = setTimeout(function () {
              button.textContent = button.getAttribute('data-label');
              button.classList.remove('is-copied');
              live.textContent = '';
            }, 2000);
          },
          function () {
            // Clipboard refused: select the command so it can be copied by hand.
            var range = document.createRange();
            range.selectNodeContents(code);
            var selection = window.getSelection();
            selection.removeAllRanges();
            selection.addRange(range);
          },
        );
      });
    });
  }

  // Install tabs. Without this script every panel shows, each with its heading.
  document.querySelectorAll('[data-tabs]').forEach(function (box) {
    var list = box.querySelector('[role="tablist"]');
    var tabs = Array.prototype.slice.call(box.querySelectorAll('[role="tab"]'));
    var panels = tabs.map(function (tab) {
      return document.getElementById(tab.getAttribute('aria-controls'));
    });
    function select(i, focus) {
      tabs.forEach(function (tab, j) {
        var on = i === j;
        tab.setAttribute('aria-selected', String(on));
        tab.classList.toggle('is-selected', on);
        tab.tabIndex = on ? 0 : -1;
        panels[j].hidden = !on;
      });
      if (focus) tabs[i].focus();
    }
    list.hidden = false;
    box.classList.add('is-tabbed');
    tabs.forEach(function (tab, i) {
      tab.addEventListener('click', function () {
        select(i, false);
      });
      tab.addEventListener('keydown', function (e) {
        var n = tabs.length;
        if (e.key === 'ArrowRight') select((i + 1) % n, true);
        else if (e.key === 'ArrowLeft') select((i - 1 + n) % n, true);
        else if (e.key === 'Home') select(0, true);
        else if (e.key === 'End') select(n - 1, true);
        else return;
        e.preventDefault();
      });
    });
    select(0, false);
  });

  // The night: the played part of the thread follows the reader down the
  // page, like the played part of the player's timeline. The site's one
  // motion; it moves only when the reader scrolls.
  document.querySelectorAll('[data-night]').forEach(function (night) {
    var played = night.querySelector('.night__played');
    var marks = Array.prototype.slice.call(night.querySelectorAll('.beat__time'));
    var queued = false;
    function update() {
      queued = false;
      var box = night.getBoundingClientRect();
      var line = window.innerHeight * 0.45;
      var h = Math.max(0, Math.min(box.height, line - box.top));
      played.style.height = h + 'px';
      marks.forEach(function (m) {
        m.classList.toggle('is-lit', m.getBoundingClientRect().top < line);
      });
    }
    function queue() {
      if (!queued) {
        queued = true;
        window.requestAnimationFrame(update);
      }
    }
    night.classList.add('is-live');
    window.addEventListener('scroll', queue, { passive: true });
    window.addEventListener('resize', queue);
    update();
  });
})();
