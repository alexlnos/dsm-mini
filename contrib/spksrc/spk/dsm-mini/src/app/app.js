// The settings screen inside DSM.
//
// Plain DOM, no framework and no build step: the page is served straight out
// of the package by DSM's web server, and anything that needed bundling would
// have to be built and committed for a screen this small.
//
// Every request goes through api.cgi, which is the only thing that can reach
// the service: it listens on loopback and a browser cannot. Authorisation is
// the service's job — see internal/dsmui.
(function () {
  'use strict';

  var DICT = window.DSMMINI_I18N || {};
  var LANG = (function () {
    var want = (navigator.language || 'en').toLowerCase();
    if (DICT[want]) return want;
    var short = want.split('-')[0];
    return DICT[short] ? short : 'en';
  })();

  function t(key) {
    var pack = DICT[LANG] || {};
    return pack[key] || (DICT.en && DICT.en[key]) || key;
  }

  // DSM's CSRF token. The screen is an iframe of the same origin as the
  // desktop that opened it, so the token can be read from there rather than
  // passed through a URL, where it would end up in logs and referrers.
  //
  // Opened on its own in a browser tab there is no desktop and no token, and
  // DSM will refuse the session — which is the honest outcome: this screen is
  // meant to be reached from the DSM menu.
  function synoToken() {
    try {
      return (window.parent && window.parent.SYNO && window.parent.SYNO.SDS &&
        window.parent.SYNO.SDS.Session && window.parent.SYNO.SDS.Session.SynoToken) || '';
    } catch (e) {
      return '';
    }
  }

  function call(endpoint, method, body) {
    return fetch('api.cgi?p=' + endpoint, {
      method: method || 'GET',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-Syno-Token': synoToken() },
      body: body ? JSON.stringify(body) : undefined
    }).then(function (r) {
      return r.json().catch(function () { return {}; }).then(function (data) {
        if (r.ok) return data;
        var err = new Error(data.error || 'request failed');
        err.status = r.status;
        err.field = data.field;
        throw err;
      });
    });
  }

  function el(tag, attrs, children) {
    var node = document.createElement(tag);
    Object.keys(attrs || {}).forEach(function (k) {
      if (k === 'text') node.textContent = attrs[k];
      else if (k === 'class') node.className = attrs[k];
      else if (k in node) node[k] = attrs[k];
      else node.setAttribute(k, attrs[k]);
    });
    (children || []).forEach(function (c) { if (c) node.appendChild(c); });
    return node;
  }

  function field(key, label, hint, value, type) {
    var input = el('input', { type: type || 'text', id: 'f-' + key, value: value || '' });
    input.dataset.key = key;
    return el('div', { class: 'row' }, [
      el('label', { text: label, for: 'f-' + key }),
      input,
      hint ? el('span', { class: 'hint', text: hint }) : null
    ]);
  }

  function radio(name, value, current, label, hint) {
    var input = el('input', { type: 'radio', name: name, value: value });
    input.checked = current === value;
    return el('label', { class: 'pick' }, [
      input,
      el('span', {}, [
        el('span', { text: label }),
        el('span', { class: 'hint', text: hint })
      ])
    ]);
  }

  function kv(label, value) {
    return el('div', { class: 'kv' }, [
      el('span', { text: label }),
      el('span', { text: value })
    ]);
  }

  var app = document.getElementById('app');
  var state = { settings: null, address: null };

  function render() {
    var s = state.settings, a = state.address;
    app.textContent = '';

    // Connection to the NAS.
    app.appendChild(el('h2', { text: t('connection') }));
    app.appendChild(el('div', { class: 'card' }, [
      el('p', { class: 'hint', text: t('connectionHint') }),
      field('DSM_URL', t('fDsmUrl'), t('hDsmUrl'), s.values.DSM_URL),
      field('DSM_USER', t('fDsmUser'), t('hDsmUser'), s.values.DSM_USER),
      field('DSM_PASSWORD', t('fDsmPassword'),
        s.set.DSM_PASSWORD ? t('secretSet') : t('secretUnset'), '', 'password'),
      field('TELEGRAM_BOT_TOKEN', t('fBotToken'),
        (s.set.TELEGRAM_BOT_TOKEN ? t('secretSet') : t('secretUnset')) + ' ' + t('hBotToken'),
        '', 'password'),
      field('ALLOWED_USER_IDS', t('fAllowedIds'), t('hAllowedIds'), s.values.ALLOWED_USER_IDS),
      field('LISTEN_ADDR', t('fListen'), t('hListen'), s.values.LISTEN_ADDR)
    ]));

    // Notifications.
    app.appendChild(el('h2', { text: t('notify') }));
    app.appendChild(el('div', { class: 'card' }, [
      el('p', { class: 'hint', text: t('notifyHint') }),
      radio('notify', 'off', s.notifications, t('notifyOff'), t('notifyOffHint')),
      radio('notify', 'downloads', s.notifications, t('notifyDown'), t('notifyDownHint')),
      radio('notify', 'all', s.notifications, t('notifyAll'), t('notifyAllHint'))
    ]));

    // Public address.
    app.appendChild(el('h2', { text: t('address') }));
    var card = el('div', { class: 'card' }, [
      el('p', { class: 'hint', text: t('addressHint') }),
      kv(t('addrCurrent'), a.public || t('addrNone')),
      kv(t('addrExternal'), a.external_ip || t('addrNone'))
    ]);

    if (a.matching) {
      card.appendChild(el('p', { class: 'note good', text: t('addrMatched') }));
      card.appendChild(kv((a.matching.https ? 'https://' : 'http://') + a.matching.fqdn,
        'localhost:' + a.matching.backend_port));
    } else {
      var name = el('input', { type: 'text', id: 'fqdn', placeholder: 'nas.example.com' });
      var create = el('button', { class: 'ghost', text: t('addrCreateBtn') });
      create.addEventListener('click', function () {
        create.disabled = true;
        call('address/proxy', 'POST', { fqdn: name.value })
          .then(load)
          .catch(showError)
          .then(function () { create.disabled = false; });
      });
      card.appendChild(el('div', { class: 'row' }, [
        el('label', { text: t('addrCreate'), for: 'fqdn' }),
        name,
        el('span', { class: 'hint', text: t('addrFqdnHint') })
      ]));
      card.appendChild(el('div', { class: 'bar' }, [create]));
    }

    card.appendChild(el('h2', { text: t('addrDdns') }));
    if (a.ddns && a.ddns.length) {
      a.ddns.forEach(function (d) { card.appendChild(kv(d.hostname, d.provider || '')); });
    } else {
      card.appendChild(el('p', { class: 'hint', text: t('addrNoDdns') }));
    }
    app.appendChild(card);

    // Save.
    var save = el('button', { text: t('save') });
    var status = el('span', { class: 'hint' });
    save.addEventListener('click', function () {
      save.disabled = true;
      status.textContent = t('saving');

      var values = {};
      app.querySelectorAll('input[data-key]').forEach(function (i) { values[i.dataset.key] = i.value; });
      var picked = app.querySelector('input[name=notify]:checked');

      call('settings', 'PUT', { values: values, notifications: picked ? picked.value : undefined })
        .then(function (saved) {
          state.settings = saved;
          render();
          note(saved.restart_needed ? 'info' : 'good',
            saved.restart_needed ? t('saved') + ' ' + t('restartNeeded') : t('saved'));
        })
        .catch(showError)
        .then(function () { save.disabled = false; });
    });
    app.appendChild(el('div', { class: 'bar' }, [save, status]));
  }

  function note(kind, text) {
    var box = el('p', { class: 'note ' + kind, text: text });
    app.appendChild(box);
    box.scrollIntoView({ block: 'nearest' });
  }

  function showError(err) {
    note('bad', err && err.status === 403 ? t('errForbidden') : (err && err.message) || t('errGeneric'));
  }

  function load() {
    return Promise.all([call('settings'), call('address')]).then(function (r) {
      state.settings = r[0];
      state.address = r[1];
      render();
    });
  }

  load().catch(function (err) {
    app.textContent = '';
    showError(err);
  });
})();
