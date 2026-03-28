/**
 * GoatFlow Accessibility Module
 *
 * Provides keyboard navigation, focus management, and WCAG 2.1 AA compliance:
 * - Skip-to-content link
 * - Focus-visible styling (keyboard vs mouse)
 * - Arrow key navigation in menus and dropdowns
 * - Escape key closes modals/dropdowns
 * - Tab trapping in modals
 * - Roving tabindex for toolbar patterns
 */

(function() {
  'use strict';

  // --- Focus-visible detection ---
  // Show focus ring only for keyboard users, not mouse clicks.
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Tab') {
      document.body.classList.add('keyboard-nav');
    }
  });
  document.addEventListener('mousedown', function() {
    document.body.classList.remove('keyboard-nav');
  });

  // --- Skip to content ---
  var skipLink = document.querySelector('.skip-to-content');
  if (skipLink) {
    skipLink.addEventListener('click', function(e) {
      e.preventDefault();
      var main = document.getElementById('main-content');
      if (main) {
        main.setAttribute('tabindex', '-1');
        main.focus();
        main.removeAttribute('tabindex');
      }
    });
  }

  // --- Escape key closes open dropdowns/modals ---
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
      // Close Alpine.js dropdowns
      document.querySelectorAll('[x-data]').forEach(function(el) {
        if (el.__x && el.__x.$data && el.__x.$data.open) {
          el.__x.$data.open = false;
        }
      });

      // Close modals
      var openModal = document.querySelector('.gk-modal[data-open="true"], .gk-modal.active');
      if (openModal) {
        openModal.style.display = 'none';
        openModal.removeAttribute('data-open');
        openModal.classList.remove('active');
      }
    }
  });

  // --- Arrow key navigation in nav menus ---
  document.querySelectorAll('[role="menubar"], [role="menu"]').forEach(function(menu) {
    menu.addEventListener('keydown', function(e) {
      var items = menu.querySelectorAll('[role="menuitem"]:not([disabled])');
      if (!items.length) return;

      var currentIdx = Array.from(items).indexOf(document.activeElement);

      switch (e.key) {
        case 'ArrowRight':
        case 'ArrowDown':
          e.preventDefault();
          items[(currentIdx + 1) % items.length].focus();
          break;
        case 'ArrowLeft':
        case 'ArrowUp':
          e.preventDefault();
          items[(currentIdx - 1 + items.length) % items.length].focus();
          break;
        case 'Home':
          e.preventDefault();
          items[0].focus();
          break;
        case 'End':
          e.preventDefault();
          items[items.length - 1].focus();
          break;
      }
    });
  });

  // --- Focus trap for modals ---
  window.trapFocus = function(modal) {
    var focusable = modal.querySelectorAll(
      'a[href], button:not([disabled]), textarea, input:not([disabled]), select, [tabindex]:not([tabindex="-1"])'
    );
    if (!focusable.length) return;

    var first = focusable[0];
    var last = focusable[focusable.length - 1];

    modal.addEventListener('keydown', function(e) {
      if (e.key !== 'Tab') return;

      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    });

    first.focus();
  };

  // --- Announce dynamic content to screen readers ---
  window.announceToSR = function(message, priority) {
    var el = document.getElementById('sr-announcer');
    if (!el) {
      el = document.createElement('div');
      el.id = 'sr-announcer';
      el.setAttribute('aria-live', priority || 'polite');
      el.setAttribute('aria-atomic', 'true');
      el.className = 'sr-only';
      document.body.appendChild(el);
    }
    el.textContent = '';
    setTimeout(function() { el.textContent = message; }, 50);
  };
})();
