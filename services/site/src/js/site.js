// Progressive enhancement only: every page reads and works without this file.
// Hooks are explicit classes and data attributes rendered by the templates;
// labels come from data attributes filled from the locale files.
(function () {
  'use strict';

  var doc = document;
  var root = doc.documentElement;
  var RM = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var $$ = function (sel, scope) {
    return Array.prototype.slice.call((scope || doc).querySelectorAll(sel));
  };

  // ---------- header: solid after the first screen, hides going down ----------
  var hdr = doc.querySelector('[data-hdr]');
  var menu = doc.querySelector('.menu');
  var prog = doc.querySelector('.prog');

  // Close the mobile menu on outside click or Escape.
  if (menu) {
    doc.addEventListener('click', function (e) {
      if (menu.open && !menu.contains(e.target)) menu.open = false;
    });
    doc.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && menu.open) {
        menu.open = false;
        menu.querySelector('summary').focus();
      }
    });
  }

  // ---------- nav hover pill ----------
  var nav = doc.querySelector('.nav');
  if (nav) {
    var pill = doc.createElement('span');
    pill.className = 'nav__pill';
    pill.setAttribute('aria-hidden', 'true');
    nav.insertBefore(pill, nav.firstChild);
    $$('.nav__link', nav).forEach(function (a) {
      a.addEventListener('mouseenter', function () {
        pill.style.left = a.offsetLeft + 'px';
        pill.style.top = a.offsetTop + 'px';
        pill.style.width = a.offsetWidth + 'px';
        pill.style.height = a.offsetHeight + 'px';
        pill.style.opacity = '1';
      });
    });
    nav.addEventListener('mouseleave', function () {
      pill.style.opacity = '0';
    });
  }

  // ---------- word-by-word headings ----------
  $$('[data-words]').forEach(function (h) {
    var i = 0;
    $$('*', h)
      .concat([h])
      .forEach(function (el) {
        Array.prototype.slice.call(el.childNodes).forEach(function (node) {
          if (node.nodeType !== 3 || !node.textContent.trim()) return;
          var frag = doc.createDocumentFragment();
          node.textContent.split(/(\s+)/).forEach(function (part) {
            if (!part) return;
            if (/^\s+$/.test(part)) {
              frag.appendChild(doc.createTextNode(part));
              return;
            }
            var span = doc.createElement('span');
            span.className = 'wd';
            span.style.setProperty('--i', String(i++));
            span.textContent = part;
            frag.appendChild(span);
          });
          node.parentNode.replaceChild(frag, node);
        });
      });
  });

  // ---------- staggered reveals ----------
  // Within each section, reveals cascade by 80 ms (at most 8 steps).
  $$('main section, main > .wrap, .foot').forEach(function (scope) {
    var n = 0;
    $$('.rv, .rvi, .hl', scope).forEach(function (el) {
      if (el.dataset.staggered) return;
      el.dataset.staggered = '1';
      if (!el.style.getPropertyValue('--d') && !el.classList.contains('banner')) {
        el.style.setProperty('--d', Math.min(n, 8) * 80 + 'ms');
      }
      n++;
    });
  });

  // ---------- typed install command ----------
  function typeCommand(code) {
    if (code.dataset.typed) return;
    code.dataset.typed = '1';
    var text = code.querySelector('.cmd__text');
    if (!text || RM) return;
    var full = text.textContent;
    var caret = doc.createElement('span');
    caret.className = 'caret';
    caret.setAttribute('aria-hidden', 'true');
    var i = 0;
    text.textContent = '';
    text.parentNode.appendChild(caret);
    var timer = setInterval(function () {
      i += 2;
      text.textContent = full.slice(0, i);
      if (i >= full.length) {
        clearInterval(timer);
        text.textContent = full;
      }
    }, 22);
  }

  // ---------- intersection: everything that waits to be seen ----------
  var targets = $$('.rv, .rvi, .hl, [data-words], .rvs, [data-devices], [data-type]');
  if ('IntersectionObserver' in window && !RM) {
    var io = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (e) {
          if (!e.isIntersecting) return;
          var t = e.target;
          t.classList.add('in');
          io.unobserve(t);
          if (t.hasAttribute('data-type')) typeCommand(t);
        });
      },
      { rootMargin: '0px 0px -12% 0px', threshold: 0 },
    );
    targets.forEach(function (t) {
      io.observe(t);
    });
  } else {
    targets.forEach(function (t) {
      t.classList.add('in');
    });
  }

  // ---------- copy buttons ----------
  if (navigator.clipboard) {
    $$('[data-copy]').forEach(function (button) {
      var block = button.closest('[data-cmd]');
      var text = block.querySelector('.cmd__text');
      var live = block.querySelector('[aria-live]');
      var full = text.textContent;
      var timer;
      button.hidden = false;
      button.addEventListener('click', function () {
        navigator.clipboard.writeText(full).then(function () {
          var done = button.getAttribute('data-copied');
          button.innerHTML =
            '<svg class="glyph" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" aria-hidden="true"><path d="M5 12l5 5L20 7"/></svg>';
          button.appendChild(doc.createTextNode(done));
          live.textContent = done;
          clearTimeout(timer);
          timer = setTimeout(function () {
            button.textContent = button.getAttribute('data-label');
            live.textContent = '';
          }, 1800);
        });
      });
    });
  }

  // ---------- home FAQ accordion (one open at a time) ----------
  var faqs = $$('[data-fq]');
  function syncFaq() {
    faqs.forEach(function (f) {
      var open = f.classList.contains('open');
      f.querySelector('.fq__q').setAttribute('aria-expanded', String(open));
      f.querySelector('.fa').inert = !open;
    });
  }
  faqs.forEach(function (f) {
    f.querySelector('.fq__q').addEventListener('click', function () {
      var was = f.classList.contains('open');
      faqs.forEach(function (o) {
        o.classList.remove('open');
      });
      if (!was) f.classList.add('open');
      syncFaq();
    });
  });
  syncFaq();

  // ---------- install methods: highlight the one jumped to ----------
  $$('[data-seg]').forEach(function (seg) {
    var items = $$('.seg__item', seg);
    items.forEach(function (a) {
      a.addEventListener('click', function () {
        items.forEach(function (o) {
          o.classList.toggle('is-on', o === a);
        });
      });
    });
  });

  // ---------- magnetic pills ----------
  if (!RM && window.matchMedia('(hover: hover)').matches) {
    $$('main .btn, .hdr .btn').forEach(function (b) {
      b.addEventListener('mousemove', function (e) {
        var r = b.getBoundingClientRect();
        var x = ((e.clientX - r.left - r.width / 2) / r.width) * 10;
        var y = ((e.clientY - r.top - r.height / 2) / r.height) * 8 - 2;
        b.style.translate = x + 'px ' + y + 'px';
      });
      b.addEventListener('mouseleave', function () {
        b.style.translate = '';
      });
    });
  }

  // ---------- home: a soft light follows the pointer over the hero ----------
  var zone = doc.querySelector('.page-home .hero-zone');
  if (zone && !RM && window.matchMedia('(hover: hover)').matches) {
    var glow = doc.createElement('div');
    glow.className = 'glow';
    glow.setAttribute('aria-hidden', 'true');
    zone.insertBefore(glow, zone.firstChild.nextSibling);
    window.addEventListener('pointermove', function (e) {
      var r = zone.getBoundingClientRect();
      var y = e.clientY - r.top;
      if (y > r.height) return;
      glow.style.left = e.clientX - r.left + 'px';
      glow.style.top = y + 'px';
    });
  }

  // ---------- page leave: the veil comes back ----------
  if (!RM) {
    doc.addEventListener('click', function (e) {
      var a = e.target.closest && e.target.closest('a[href]');
      if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      if (a.target && a.target !== '_self') return;
      var url = new URL(a.href, location.href);
      if (url.origin !== location.origin || a.hasAttribute('download')) return;
      if (url.pathname === location.pathname && url.hash) return;
      if (url.href === location.href) return;
      e.preventDefault();
      doc.body.classList.add('leaving');
      setTimeout(function () {
        location.href = url.href;
      }, 420);
    });
    window.addEventListener('pageshow', function (e) {
      if (e.persisted) doc.body.classList.remove('leaving');
    });
  }

  // ---------- scroll loop ----------
  var tilt = doc.querySelector('[data-tilt]');
  var field = doc.querySelector('[data-ambient]');
  var hero = doc.querySelector('.hero');
  var banners = $$('.banner img');
  var atvBg = doc.querySelector('.atv__bg img');
  var lastY = window.scrollY;
  var ticking = false;

  function frame() {
    ticking = false;
    var y = window.scrollY;
    var vh = window.innerHeight;
    var max = root.scrollHeight - vh;
    if (prog) prog.style.transform = 'scaleX(' + (max > 0 ? y / max : 0) + ')';
    if (hdr) {
      hdr.classList.toggle('is-solid', y > 40);
      var menuOpen = menu && menu.open;
      if (!menuOpen && y > 500 && y > lastY + 4) hdr.classList.add('is-away');
      else if (y < lastY - 4 || y < 500 || menuOpen) hdr.classList.remove('is-away');
    }
    lastY = y;
    if (RM) return;

    if (tilt) {
      var r = tilt.getBoundingClientRect();
      var p = Math.max(0, Math.min(1, 1 - (r.top - vh * 0.25) / (vh * 0.75)));
      tilt.style.transform = 'perspective(2400px) rotateX(' + (1 - p) * 16 + 'deg) scale(' + (0.9 + p * 0.1) + ')';
    }
    if (field && y < vh * 1.6) field.style.translate = '0 ' + y * 0.35 + 'px';
    if (hero && y < vh * 1.2) {
      var k = Math.min(1, y / (vh * 0.9));
      hero.style.opacity = String(1 - k * 0.9);
      hero.style.translate = '0 ' + y * 0.25 + 'px';
      hero.style.filter = k > 0.01 ? 'blur(' + k * 8 + 'px)' : '';
    }
    banners.forEach(function (img) {
      var b = img.parentNode.getBoundingClientRect();
      if (b.bottom < 0 || b.top > vh) return;
      img.style.translate = '0 ' + ((b.top + b.height / 2 - vh / 2) / vh) * -60 + 'px';
    });
    if (atvBg) {
      var s = atvBg.parentNode.getBoundingClientRect();
      if (s.bottom > 0 && s.top < vh) atvBg.style.translate = '0 ' + ((s.top + s.height / 2 - vh / 2) / vh) * -70 + 'px';
    }
  }

  window.addEventListener(
    'scroll',
    function () {
      if (!ticking) {
        ticking = true;
        requestAnimationFrame(frame);
      }
    },
    { passive: true },
  );
  if (menu) menu.addEventListener('toggle', frame);
  frame();
})();
