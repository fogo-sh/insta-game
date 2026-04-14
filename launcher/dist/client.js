"use strict";
(() => {
  // node_modules/preact/dist/preact.module.js
  var n;
  var l;
  var u;
  var t;
  var i;
  var r;
  var o;
  var e;
  var f;
  var c;
  var s;
  var a;
  var h;
  var p;
  var v;
  var y;
  var d = {};
  var w = [];
  var _ = /acit|ex(?:s|g|n|p|$)|rph|grid|ows|mnc|ntw|ine[ch]|zoo|^ord|itera/i;
  var g = Array.isArray;
  function m(n2, l3) {
    for (var u4 in l3) n2[u4] = l3[u4];
    return n2;
  }
  function b(n2) {
    n2 && n2.parentNode && n2.parentNode.removeChild(n2);
  }
  function k(l3, u4, t3) {
    var i3, r3, o3, e3 = {};
    for (o3 in u4) "key" == o3 ? i3 = u4[o3] : "ref" == o3 ? r3 = u4[o3] : e3[o3] = u4[o3];
    if (arguments.length > 2 && (e3.children = arguments.length > 3 ? n.call(arguments, 2) : t3), "function" == typeof l3 && null != l3.defaultProps) for (o3 in l3.defaultProps) void 0 === e3[o3] && (e3[o3] = l3.defaultProps[o3]);
    return x(l3, e3, i3, r3, null);
  }
  function x(n2, t3, i3, r3, o3) {
    var e3 = { type: n2, props: t3, key: i3, ref: r3, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: null == o3 ? ++u : o3, __i: -1, __u: 0 };
    return null == o3 && null != l.vnode && l.vnode(e3), e3;
  }
  function S(n2) {
    return n2.children;
  }
  function C(n2, l3) {
    this.props = n2, this.context = l3;
  }
  function $(n2, l3) {
    if (null == l3) return n2.__ ? $(n2.__, n2.__i + 1) : null;
    for (var u4; l3 < n2.__k.length; l3++) if (null != (u4 = n2.__k[l3]) && null != u4.__e) return u4.__e;
    return "function" == typeof n2.type ? $(n2) : null;
  }
  function I(n2) {
    if (n2.__P && n2.__d) {
      var u4 = n2.__v, t3 = u4.__e, i3 = [], r3 = [], o3 = m({}, u4);
      o3.__v = u4.__v + 1, l.vnode && l.vnode(o3), q(n2.__P, o3, u4, n2.__n, n2.__P.namespaceURI, 32 & u4.__u ? [t3] : null, i3, null == t3 ? $(u4) : t3, !!(32 & u4.__u), r3), o3.__v = u4.__v, o3.__.__k[o3.__i] = o3, D(i3, o3, r3), u4.__e = u4.__ = null, o3.__e != t3 && P(o3);
    }
  }
  function P(n2) {
    if (null != (n2 = n2.__) && null != n2.__c) return n2.__e = n2.__c.base = null, n2.__k.some(function(l3) {
      if (null != l3 && null != l3.__e) return n2.__e = n2.__c.base = l3.__e;
    }), P(n2);
  }
  function A(n2) {
    (!n2.__d && (n2.__d = true) && i.push(n2) && !H.__r++ || r != l.debounceRendering) && ((r = l.debounceRendering) || o)(H);
  }
  function H() {
    try {
      for (var n2, l3 = 1; i.length; ) i.length > l3 && i.sort(e), n2 = i.shift(), l3 = i.length, I(n2);
    } finally {
      i.length = H.__r = 0;
    }
  }
  function L(n2, l3, u4, t3, i3, r3, o3, e3, f4, c3, s3) {
    var a3, h3, p3, v3, y3, _2, g2, m3 = t3 && t3.__k || w, b2 = l3.length;
    for (f4 = T(u4, l3, m3, f4, b2), a3 = 0; a3 < b2; a3++) null != (p3 = u4.__k[a3]) && (h3 = -1 != p3.__i && m3[p3.__i] || d, p3.__i = a3, _2 = q(n2, p3, h3, i3, r3, o3, e3, f4, c3, s3), v3 = p3.__e, p3.ref && h3.ref != p3.ref && (h3.ref && J(h3.ref, null, p3), s3.push(p3.ref, p3.__c || v3, p3)), null == y3 && null != v3 && (y3 = v3), (g2 = !!(4 & p3.__u)) || h3.__k === p3.__k ? (f4 = j(p3, f4, n2, g2), g2 && h3.__e && (h3.__e = null)) : "function" == typeof p3.type && void 0 !== _2 ? f4 = _2 : v3 && (f4 = v3.nextSibling), p3.__u &= -7);
    return u4.__e = y3, f4;
  }
  function T(n2, l3, u4, t3, i3) {
    var r3, o3, e3, f4, c3, s3 = u4.length, a3 = s3, h3 = 0;
    for (n2.__k = new Array(i3), r3 = 0; r3 < i3; r3++) null != (o3 = l3[r3]) && "boolean" != typeof o3 && "function" != typeof o3 ? ("string" == typeof o3 || "number" == typeof o3 || "bigint" == typeof o3 || o3.constructor == String ? o3 = n2.__k[r3] = x(null, o3, null, null, null) : g(o3) ? o3 = n2.__k[r3] = x(S, { children: o3 }, null, null, null) : void 0 === o3.constructor && o3.__b > 0 ? o3 = n2.__k[r3] = x(o3.type, o3.props, o3.key, o3.ref ? o3.ref : null, o3.__v) : n2.__k[r3] = o3, f4 = r3 + h3, o3.__ = n2, o3.__b = n2.__b + 1, e3 = null, -1 != (c3 = o3.__i = O(o3, u4, f4, a3)) && (a3--, (e3 = u4[c3]) && (e3.__u |= 2)), null == e3 || null == e3.__v ? (-1 == c3 && (i3 > s3 ? h3-- : i3 < s3 && h3++), "function" != typeof o3.type && (o3.__u |= 4)) : c3 != f4 && (c3 == f4 - 1 ? h3-- : c3 == f4 + 1 ? h3++ : (c3 > f4 ? h3-- : h3++, o3.__u |= 4))) : n2.__k[r3] = null;
    if (a3) for (r3 = 0; r3 < s3; r3++) null != (e3 = u4[r3]) && 0 == (2 & e3.__u) && (e3.__e == t3 && (t3 = $(e3)), K(e3, e3));
    return t3;
  }
  function j(n2, l3, u4, t3) {
    var i3, r3;
    if ("function" == typeof n2.type) {
      for (i3 = n2.__k, r3 = 0; i3 && r3 < i3.length; r3++) i3[r3] && (i3[r3].__ = n2, l3 = j(i3[r3], l3, u4, t3));
      return l3;
    }
    n2.__e != l3 && (t3 && (l3 && n2.type && !l3.parentNode && (l3 = $(n2)), u4.insertBefore(n2.__e, l3 || null)), l3 = n2.__e);
    do {
      l3 = l3 && l3.nextSibling;
    } while (null != l3 && 8 == l3.nodeType);
    return l3;
  }
  function O(n2, l3, u4, t3) {
    var i3, r3, o3, e3 = n2.key, f4 = n2.type, c3 = l3[u4], s3 = null != c3 && 0 == (2 & c3.__u);
    if (null === c3 && null == e3 || s3 && e3 == c3.key && f4 == c3.type) return u4;
    if (t3 > (s3 ? 1 : 0)) {
      for (i3 = u4 - 1, r3 = u4 + 1; i3 >= 0 || r3 < l3.length; ) if (null != (c3 = l3[o3 = i3 >= 0 ? i3-- : r3++]) && 0 == (2 & c3.__u) && e3 == c3.key && f4 == c3.type) return o3;
    }
    return -1;
  }
  function z(n2, l3, u4) {
    "-" == l3[0] ? n2.setProperty(l3, null == u4 ? "" : u4) : n2[l3] = null == u4 ? "" : "number" != typeof u4 || _.test(l3) ? u4 : u4 + "px";
  }
  function N(n2, l3, u4, t3, i3) {
    var r3, o3;
    n: if ("style" == l3) if ("string" == typeof u4) n2.style.cssText = u4;
    else {
      if ("string" == typeof t3 && (n2.style.cssText = t3 = ""), t3) for (l3 in t3) u4 && l3 in u4 || z(n2.style, l3, "");
      if (u4) for (l3 in u4) t3 && u4[l3] == t3[l3] || z(n2.style, l3, u4[l3]);
    }
    else if ("o" == l3[0] && "n" == l3[1]) r3 = l3 != (l3 = l3.replace(a, "$1")), o3 = l3.toLowerCase(), l3 = o3 in n2 || "onFocusOut" == l3 || "onFocusIn" == l3 ? o3.slice(2) : l3.slice(2), n2.l || (n2.l = {}), n2.l[l3 + r3] = u4, u4 ? t3 ? u4[s] = t3[s] : (u4[s] = h, n2.addEventListener(l3, r3 ? v : p, r3)) : n2.removeEventListener(l3, r3 ? v : p, r3);
    else {
      if ("http://www.w3.org/2000/svg" == i3) l3 = l3.replace(/xlink(H|:h)/, "h").replace(/sName$/, "s");
      else if ("width" != l3 && "height" != l3 && "href" != l3 && "list" != l3 && "form" != l3 && "tabIndex" != l3 && "download" != l3 && "rowSpan" != l3 && "colSpan" != l3 && "role" != l3 && "popover" != l3 && l3 in n2) try {
        n2[l3] = null == u4 ? "" : u4;
        break n;
      } catch (n3) {
      }
      "function" == typeof u4 || (null == u4 || false === u4 && "-" != l3[4] ? n2.removeAttribute(l3) : n2.setAttribute(l3, "popover" == l3 && 1 == u4 ? "" : u4));
    }
  }
  function V(n2) {
    return function(u4) {
      if (this.l) {
        var t3 = this.l[u4.type + n2];
        if (null == u4[c]) u4[c] = h++;
        else if (u4[c] < t3[s]) return;
        return t3(l.event ? l.event(u4) : u4);
      }
    };
  }
  function q(n2, u4, t3, i3, r3, o3, e3, f4, c3, s3) {
    var a3, h3, p3, v3, y3, d3, _2, k3, x2, M, $2, I2, P2, A3, H2, T3 = u4.type;
    if (void 0 !== u4.constructor) return null;
    128 & t3.__u && (c3 = !!(32 & t3.__u), o3 = [f4 = u4.__e = t3.__e]), (a3 = l.__b) && a3(u4);
    n: if ("function" == typeof T3) try {
      if (k3 = u4.props, x2 = T3.prototype && T3.prototype.render, M = (a3 = T3.contextType) && i3[a3.__c], $2 = a3 ? M ? M.props.value : a3.__ : i3, t3.__c ? _2 = (h3 = u4.__c = t3.__c).__ = h3.__E : (x2 ? u4.__c = h3 = new T3(k3, $2) : (u4.__c = h3 = new C(k3, $2), h3.constructor = T3, h3.render = Q), M && M.sub(h3), h3.state || (h3.state = {}), h3.__n = i3, p3 = h3.__d = true, h3.__h = [], h3._sb = []), x2 && null == h3.__s && (h3.__s = h3.state), x2 && null != T3.getDerivedStateFromProps && (h3.__s == h3.state && (h3.__s = m({}, h3.__s)), m(h3.__s, T3.getDerivedStateFromProps(k3, h3.__s))), v3 = h3.props, y3 = h3.state, h3.__v = u4, p3) x2 && null == T3.getDerivedStateFromProps && null != h3.componentWillMount && h3.componentWillMount(), x2 && null != h3.componentDidMount && h3.__h.push(h3.componentDidMount);
      else {
        if (x2 && null == T3.getDerivedStateFromProps && k3 !== v3 && null != h3.componentWillReceiveProps && h3.componentWillReceiveProps(k3, $2), u4.__v == t3.__v || !h3.__e && null != h3.shouldComponentUpdate && false === h3.shouldComponentUpdate(k3, h3.__s, $2)) {
          u4.__v != t3.__v && (h3.props = k3, h3.state = h3.__s, h3.__d = false), u4.__e = t3.__e, u4.__k = t3.__k, u4.__k.some(function(n3) {
            n3 && (n3.__ = u4);
          }), w.push.apply(h3.__h, h3._sb), h3._sb = [], h3.__h.length && e3.push(h3);
          break n;
        }
        null != h3.componentWillUpdate && h3.componentWillUpdate(k3, h3.__s, $2), x2 && null != h3.componentDidUpdate && h3.__h.push(function() {
          h3.componentDidUpdate(v3, y3, d3);
        });
      }
      if (h3.context = $2, h3.props = k3, h3.__P = n2, h3.__e = false, I2 = l.__r, P2 = 0, x2) h3.state = h3.__s, h3.__d = false, I2 && I2(u4), a3 = h3.render(h3.props, h3.state, h3.context), w.push.apply(h3.__h, h3._sb), h3._sb = [];
      else do {
        h3.__d = false, I2 && I2(u4), a3 = h3.render(h3.props, h3.state, h3.context), h3.state = h3.__s;
      } while (h3.__d && ++P2 < 25);
      h3.state = h3.__s, null != h3.getChildContext && (i3 = m(m({}, i3), h3.getChildContext())), x2 && !p3 && null != h3.getSnapshotBeforeUpdate && (d3 = h3.getSnapshotBeforeUpdate(v3, y3)), A3 = null != a3 && a3.type === S && null == a3.key ? E(a3.props.children) : a3, f4 = L(n2, g(A3) ? A3 : [A3], u4, t3, i3, r3, o3, e3, f4, c3, s3), h3.base = u4.__e, u4.__u &= -161, h3.__h.length && e3.push(h3), _2 && (h3.__E = h3.__ = null);
    } catch (n3) {
      if (u4.__v = null, c3 || null != o3) if (n3.then) {
        for (u4.__u |= c3 ? 160 : 128; f4 && 8 == f4.nodeType && f4.nextSibling; ) f4 = f4.nextSibling;
        o3[o3.indexOf(f4)] = null, u4.__e = f4;
      } else {
        for (H2 = o3.length; H2--; ) b(o3[H2]);
        B(u4);
      }
      else u4.__e = t3.__e, u4.__k = t3.__k, n3.then || B(u4);
      l.__e(n3, u4, t3);
    }
    else null == o3 && u4.__v == t3.__v ? (u4.__k = t3.__k, u4.__e = t3.__e) : f4 = u4.__e = G(t3.__e, u4, t3, i3, r3, o3, e3, c3, s3);
    return (a3 = l.diffed) && a3(u4), 128 & u4.__u ? void 0 : f4;
  }
  function B(n2) {
    n2 && (n2.__c && (n2.__c.__e = true), n2.__k && n2.__k.some(B));
  }
  function D(n2, u4, t3) {
    for (var i3 = 0; i3 < t3.length; i3++) J(t3[i3], t3[++i3], t3[++i3]);
    l.__c && l.__c(u4, n2), n2.some(function(u5) {
      try {
        n2 = u5.__h, u5.__h = [], n2.some(function(n3) {
          n3.call(u5);
        });
      } catch (n3) {
        l.__e(n3, u5.__v);
      }
    });
  }
  function E(n2) {
    return "object" != typeof n2 || null == n2 || n2.__b > 0 ? n2 : g(n2) ? n2.map(E) : m({}, n2);
  }
  function G(u4, t3, i3, r3, o3, e3, f4, c3, s3) {
    var a3, h3, p3, v3, y3, w3, _2, m3 = i3.props || d, k3 = t3.props, x2 = t3.type;
    if ("svg" == x2 ? o3 = "http://www.w3.org/2000/svg" : "math" == x2 ? o3 = "http://www.w3.org/1998/Math/MathML" : o3 || (o3 = "http://www.w3.org/1999/xhtml"), null != e3) {
      for (a3 = 0; a3 < e3.length; a3++) if ((y3 = e3[a3]) && "setAttribute" in y3 == !!x2 && (x2 ? y3.localName == x2 : 3 == y3.nodeType)) {
        u4 = y3, e3[a3] = null;
        break;
      }
    }
    if (null == u4) {
      if (null == x2) return document.createTextNode(k3);
      u4 = document.createElementNS(o3, x2, k3.is && k3), c3 && (l.__m && l.__m(t3, e3), c3 = false), e3 = null;
    }
    if (null == x2) m3 === k3 || c3 && u4.data == k3 || (u4.data = k3);
    else {
      if (e3 = e3 && n.call(u4.childNodes), !c3 && null != e3) for (m3 = {}, a3 = 0; a3 < u4.attributes.length; a3++) m3[(y3 = u4.attributes[a3]).name] = y3.value;
      for (a3 in m3) y3 = m3[a3], "dangerouslySetInnerHTML" == a3 ? p3 = y3 : "children" == a3 || a3 in k3 || "value" == a3 && "defaultValue" in k3 || "checked" == a3 && "defaultChecked" in k3 || N(u4, a3, null, y3, o3);
      for (a3 in k3) y3 = k3[a3], "children" == a3 ? v3 = y3 : "dangerouslySetInnerHTML" == a3 ? h3 = y3 : "value" == a3 ? w3 = y3 : "checked" == a3 ? _2 = y3 : c3 && "function" != typeof y3 || m3[a3] === y3 || N(u4, a3, y3, m3[a3], o3);
      if (h3) c3 || p3 && (h3.__html == p3.__html || h3.__html == u4.innerHTML) || (u4.innerHTML = h3.__html), t3.__k = [];
      else if (p3 && (u4.innerHTML = ""), L("template" == t3.type ? u4.content : u4, g(v3) ? v3 : [v3], t3, i3, r3, "foreignObject" == x2 ? "http://www.w3.org/1999/xhtml" : o3, e3, f4, e3 ? e3[0] : i3.__k && $(i3, 0), c3, s3), null != e3) for (a3 = e3.length; a3--; ) b(e3[a3]);
      c3 || (a3 = "value", "progress" == x2 && null == w3 ? u4.removeAttribute("value") : null != w3 && (w3 !== u4[a3] || "progress" == x2 && !w3 || "option" == x2 && w3 != m3[a3]) && N(u4, a3, w3, m3[a3], o3), a3 = "checked", null != _2 && _2 != u4[a3] && N(u4, a3, _2, m3[a3], o3));
    }
    return u4;
  }
  function J(n2, u4, t3) {
    try {
      if ("function" == typeof n2) {
        var i3 = "function" == typeof n2.__u;
        i3 && n2.__u(), i3 && null == u4 || (n2.__u = n2(u4));
      } else n2.current = u4;
    } catch (n3) {
      l.__e(n3, t3);
    }
  }
  function K(n2, u4, t3) {
    var i3, r3;
    if (l.unmount && l.unmount(n2), (i3 = n2.ref) && (i3.current && i3.current != n2.__e || J(i3, null, u4)), null != (i3 = n2.__c)) {
      if (i3.componentWillUnmount) try {
        i3.componentWillUnmount();
      } catch (n3) {
        l.__e(n3, u4);
      }
      i3.base = i3.__P = null;
    }
    if (i3 = n2.__k) for (r3 = 0; r3 < i3.length; r3++) i3[r3] && K(i3[r3], u4, t3 || "function" != typeof n2.type);
    t3 || b(n2.__e), n2.__c = n2.__ = n2.__e = void 0;
  }
  function Q(n2, l3, u4) {
    return this.constructor(n2, u4);
  }
  function R(u4, t3, i3) {
    var r3, o3, e3, f4;
    t3 == document && (t3 = document.documentElement), l.__ && l.__(u4, t3), o3 = (r3 = "function" == typeof i3) ? null : i3 && i3.__k || t3.__k, e3 = [], f4 = [], q(t3, u4 = (!r3 && i3 || t3).__k = k(S, null, [u4]), o3 || d, d, t3.namespaceURI, !r3 && i3 ? [i3] : o3 ? null : t3.firstChild ? n.call(t3.childNodes) : null, e3, !r3 && i3 ? i3 : o3 ? o3.__e : t3.firstChild, r3, f4), D(e3, u4, f4);
  }
  n = w.slice, l = { __e: function(n2, l3, u4, t3) {
    for (var i3, r3, o3; l3 = l3.__; ) if ((i3 = l3.__c) && !i3.__) try {
      if ((r3 = i3.constructor) && null != r3.getDerivedStateFromError && (i3.setState(r3.getDerivedStateFromError(n2)), o3 = i3.__d), null != i3.componentDidCatch && (i3.componentDidCatch(n2, t3 || {}), o3 = i3.__d), o3) return i3.__E = i3;
    } catch (l4) {
      n2 = l4;
    }
    throw n2;
  } }, u = 0, t = function(n2) {
    return null != n2 && void 0 === n2.constructor;
  }, C.prototype.setState = function(n2, l3) {
    var u4;
    u4 = null != this.__s && this.__s != this.state ? this.__s : this.__s = m({}, this.state), "function" == typeof n2 && (n2 = n2(m({}, u4), this.props)), n2 && m(u4, n2), null != n2 && this.__v && (l3 && this._sb.push(l3), A(this));
  }, C.prototype.forceUpdate = function(n2) {
    this.__v && (this.__e = true, n2 && this.__h.push(n2), A(this));
  }, C.prototype.render = S, i = [], o = "function" == typeof Promise ? Promise.prototype.then.bind(Promise.resolve()) : setTimeout, e = function(n2, l3) {
    return n2.__v.__b - l3.__v.__b;
  }, H.__r = 0, f = Math.random().toString(8), c = "__d" + f, s = "__a" + f, a = /(PointerCapture)$|Capture$/i, h = 0, p = V(false), v = V(true), y = 0;

  // node_modules/preact/hooks/dist/hooks.module.js
  var t2;
  var r2;
  var u2;
  var i2;
  var o2 = 0;
  var f2 = [];
  var c2 = l;
  var e2 = c2.__b;
  var a2 = c2.__r;
  var v2 = c2.diffed;
  var l2 = c2.__c;
  var m2 = c2.unmount;
  var s2 = c2.__;
  function p2(n2, t3) {
    c2.__h && c2.__h(r2, n2, o2 || t3), o2 = 0;
    var u4 = r2.__H || (r2.__H = { __: [], __h: [] });
    return n2 >= u4.__.length && u4.__.push({}), u4.__[n2];
  }
  function d2(n2) {
    return o2 = 1, h2(D2, n2);
  }
  function h2(n2, u4, i3) {
    var o3 = p2(t2++, 2);
    if (o3.t = n2, !o3.__c && (o3.__ = [i3 ? i3(u4) : D2(void 0, u4), function(n3) {
      var t3 = o3.__N ? o3.__N[0] : o3.__[0], r3 = o3.t(t3, n3);
      t3 !== r3 && (o3.__N = [r3, o3.__[1]], o3.__c.setState({}));
    }], o3.__c = r2, !r2.__f)) {
      var f4 = function(n3, t3, r3) {
        if (!o3.__c.__H) return true;
        var u5 = o3.__c.__H.__.filter(function(n4) {
          return n4.__c;
        });
        if (u5.every(function(n4) {
          return !n4.__N;
        })) return !c3 || c3.call(this, n3, t3, r3);
        var i4 = o3.__c.props !== n3;
        return u5.some(function(n4) {
          if (n4.__N) {
            var t4 = n4.__[0];
            n4.__ = n4.__N, n4.__N = void 0, t4 !== n4.__[0] && (i4 = true);
          }
        }), c3 && c3.call(this, n3, t3, r3) || i4;
      };
      r2.__f = true;
      var c3 = r2.shouldComponentUpdate, e3 = r2.componentWillUpdate;
      r2.componentWillUpdate = function(n3, t3, r3) {
        if (this.__e) {
          var u5 = c3;
          c3 = void 0, f4(n3, t3, r3), c3 = u5;
        }
        e3 && e3.call(this, n3, t3, r3);
      }, r2.shouldComponentUpdate = f4;
    }
    return o3.__N || o3.__;
  }
  function y2(n2, u4) {
    var i3 = p2(t2++, 3);
    !c2.__s && C2(i3.__H, u4) && (i3.__ = n2, i3.u = u4, r2.__H.__h.push(i3));
  }
  function A2(n2) {
    return o2 = 5, T2(function() {
      return { current: n2 };
    }, []);
  }
  function T2(n2, r3) {
    var u4 = p2(t2++, 7);
    return C2(u4.__H, r3) && (u4.__ = n2(), u4.__H = r3, u4.__h = n2), u4.__;
  }
  function q2(n2, t3) {
    return o2 = 8, T2(function() {
      return n2;
    }, t3);
  }
  function j2() {
    for (var n2; n2 = f2.shift(); ) {
      var t3 = n2.__H;
      if (n2.__P && t3) try {
        t3.__h.some(z2), t3.__h.some(B2), t3.__h = [];
      } catch (r3) {
        t3.__h = [], c2.__e(r3, n2.__v);
      }
    }
  }
  c2.__b = function(n2) {
    r2 = null, e2 && e2(n2);
  }, c2.__ = function(n2, t3) {
    n2 && t3.__k && t3.__k.__m && (n2.__m = t3.__k.__m), s2 && s2(n2, t3);
  }, c2.__r = function(n2) {
    a2 && a2(n2), t2 = 0;
    var i3 = (r2 = n2.__c).__H;
    i3 && (u2 === r2 ? (i3.__h = [], r2.__h = [], i3.__.some(function(n3) {
      n3.__N && (n3.__ = n3.__N), n3.u = n3.__N = void 0;
    })) : (i3.__h.some(z2), i3.__h.some(B2), i3.__h = [], t2 = 0)), u2 = r2;
  }, c2.diffed = function(n2) {
    v2 && v2(n2);
    var t3 = n2.__c;
    t3 && t3.__H && (t3.__H.__h.length && (1 !== f2.push(t3) && i2 === c2.requestAnimationFrame || ((i2 = c2.requestAnimationFrame) || w2)(j2)), t3.__H.__.some(function(n3) {
      n3.u && (n3.__H = n3.u), n3.u = void 0;
    })), u2 = r2 = null;
  }, c2.__c = function(n2, t3) {
    t3.some(function(n3) {
      try {
        n3.__h.some(z2), n3.__h = n3.__h.filter(function(n4) {
          return !n4.__ || B2(n4);
        });
      } catch (r3) {
        t3.some(function(n4) {
          n4.__h && (n4.__h = []);
        }), t3 = [], c2.__e(r3, n3.__v);
      }
    }), l2 && l2(n2, t3);
  }, c2.unmount = function(n2) {
    m2 && m2(n2);
    var t3, r3 = n2.__c;
    r3 && r3.__H && (r3.__H.__.some(function(n3) {
      try {
        z2(n3);
      } catch (n4) {
        t3 = n4;
      }
    }), r3.__H = void 0, t3 && c2.__e(t3, r3.__v));
  };
  var k2 = "function" == typeof requestAnimationFrame;
  function w2(n2) {
    var t3, r3 = function() {
      clearTimeout(u4), k2 && cancelAnimationFrame(t3), setTimeout(n2);
    }, u4 = setTimeout(r3, 35);
    k2 && (t3 = requestAnimationFrame(r3));
  }
  function z2(n2) {
    var t3 = r2, u4 = n2.__c;
    "function" == typeof u4 && (n2.__c = void 0, u4()), r2 = t3;
  }
  function B2(n2) {
    var t3 = r2;
    n2.__c = n2.__(), r2 = t3;
  }
  function C2(n2, t3) {
    return !n2 || n2.length !== t3.length || t3.some(function(t4, r3) {
      return t4 !== n2[r3];
    });
  }
  function D2(n2, t3) {
    return "function" == typeof t3 ? t3(n2) : t3;
  }

  // src/client/api.ts
  async function fetchStatus() {
    const res = await fetch("/status");
    if (!res.ok) throw new Error(`/status returned ${res.status}`);
    return res.json();
  }
  async function validatePassphrase(passphrase) {
    const res = await fetch("/", {
      headers: { "X-Passphrase": passphrase, "X-Validate": "true" }
    });
    return res.ok;
  }
  async function postAction(game, operation, passphrase) {
    const res = await fetch(`/?game=${encodeURIComponent(game)}&operation=${operation}`, {
      method: "POST",
      headers: { "X-Passphrase": passphrase }
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `${operation} returned ${res.status}`);
    }
    return res.json();
  }
  async function pollLogs(game, token, cursor) {
    const params = new URLSearchParams({ game, token });
    if (cursor) params.set("cursor", cursor);
    const res = await fetch(`/logs?${params}`);
    if (!res.ok) throw new Error(`/logs returned ${res.status}`);
    return res.json();
  }

  // node_modules/preact/jsx-runtime/dist/jsxRuntime.module.js
  var f3 = 0;
  function u3(e3, t3, n2, o3, i3, u4) {
    t3 || (t3 = {});
    var a3, c3, p3 = t3;
    if ("ref" in p3) for (c3 in p3 = {}, t3) "ref" == c3 ? a3 = t3[c3] : p3[c3] = t3[c3];
    var l3 = { type: e3, props: p3, key: n2, ref: a3, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: --f3, __i: -1, __u: 0, __source: i3, __self: u4 };
    if ("function" == typeof e3 && (a3 = e3.defaultProps)) for (c3 in a3) void 0 === p3[c3] && (p3[c3] = a3[c3]);
    return l.vnode && l.vnode(l3), l3;
  }

  // src/client/LogPanel.tsx
  function LogPanel({ game, passphrase }) {
    const [lines, setLines] = d2([`[connecting to ${game} logs...]`]);
    const [logMode, setLogMode] = d2(null);
    const panelRef = A2(null);
    const cursorRef = A2(null);
    const activeRef = A2(true);
    y2(() => {
      const el = panelRef.current;
      if (el) el.scrollTop = el.scrollHeight;
    }, [lines]);
    y2(() => {
      activeRef.current = true;
      fetch(`/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(passphrase)}`, {
        method: "HEAD"
      }).then((res) => {
        if (!activeRef.current) return;
        const mode = res.headers.get("X-Log-Mode") === "poll" ? "poll" : "sse";
        setLogMode(mode);
      }).catch(() => {
        if (activeRef.current) setLines((l3) => [...l3, "[failed to connect]"]);
      });
      return () => {
        activeRef.current = false;
      };
    }, [game, passphrase]);
    y2(() => {
      if (logMode !== "sse") return;
      const url = `/logs?game=${encodeURIComponent(game)}&token=${encodeURIComponent(passphrase)}`;
      const es = new EventSource(url);
      es.addEventListener("log", (e3) => {
        if (!activeRef.current) return;
        setLines((l3) => [...l3, e3.data]);
      });
      es.onerror = () => {
        if (activeRef.current) setLines((l3) => [...l3, "[log stream disconnected]"]);
        es.close();
      };
      return () => {
        es.close();
      };
    }, [logMode, game, passphrase]);
    y2(() => {
      if (logMode !== "poll") return;
      let stopped = false;
      async function doPoll() {
        while (!stopped && activeRef.current) {
          try {
            const result = await pollLogs(game, passphrase, cursorRef.current);
            if (result.lines.length > 0) {
              setLines((l3) => [...l3, ...result.lines]);
            }
            if (result.cursor) cursorRef.current = result.cursor;
          } catch (err) {
            if (!stopped) setLines((l3) => [...l3, `[poll error: ${err instanceof Error ? err.message : String(err)}]`]);
          }
          await new Promise((r3) => setTimeout(r3, 5e3));
        }
      }
      void doPoll();
      return () => {
        stopped = true;
      };
    }, [logMode, game, passphrase]);
    return /* @__PURE__ */ u3("div", { class: "log-panel", ref: panelRef, children: lines.map((line, i3) => /* @__PURE__ */ u3(
      "div",
      {
        class: "log-line",
        dangerouslySetInnerHTML: { __html: line }
      },
      i3
    )) });
  }

  // src/client/GameRow.tsx
  function statusDot(status) {
    if (status === "online") return "\u{1F7E2}";
    if (status === "starting") return "\u{1F7E1}";
    return "\u26AB";
  }
  function GameRow({ id, game, passphrase, onAction }) {
    const [open, setOpen] = d2(false);
    const [logsOpen, setLogsOpen] = d2(false);
    const [actionResult, setActionResult] = d2(null);
    const [acting, setActing] = d2(false);
    const expandable = game.status !== "offline" || passphrase !== null;
    const toggle = q2(() => {
      if (!expandable) return;
      setOpen((o3) => !o3);
    }, [expandable]);
    const handleAction = q2(async (operation) => {
      if (!passphrase) return;
      setActing(true);
      setActionResult(null);
      onAction();
      try {
        const result = await postAction(id, operation, passphrase);
        setActionResult({ message: `${operation} \u2192 ${result.status}`, ok: result.status !== "offline" });
      } catch (err) {
        setActionResult({ message: `${operation} failed: ${err instanceof Error ? err.message : String(err)}`, ok: false });
      } finally {
        setActing(false);
      }
    }, [id, passphrase, onAction]);
    const copyConnect = q2((address, e3) => {
      void navigator.clipboard.writeText(address);
      const btn = e3.currentTarget;
      const orig = btn.textContent;
      btn.textContent = "\u2713";
      btn.classList.add("copied");
      setTimeout(() => {
        btn.textContent = orig;
        btn.classList.remove("copied");
      }, 1500);
    }, []);
    return /* @__PURE__ */ u3("div", { class: "row", children: [
      /* @__PURE__ */ u3(
        "div",
        {
          class: `row-header${expandable ? "" : " not-expandable"}`,
          onClick: toggle,
          children: [
            /* @__PURE__ */ u3("span", { class: "status-dot", children: statusDot(game.status) }),
            /* @__PURE__ */ u3("span", { class: "game-name", children: game.displayName || id }),
            /* @__PURE__ */ u3("span", { class: "row-meta", children: [
              game.status === "online" && game.hostname ? /* @__PURE__ */ u3("span", { children: game.hostname }) : null,
              game.status === "online" && game.map ? /* @__PURE__ */ u3("span", { children: game.map }) : null,
              game.status === "online" ? /* @__PURE__ */ u3("span", { children: [
                game.players,
                " player",
                game.players !== 1 ? "s" : ""
              ] }) : /* @__PURE__ */ u3("span", { class: game.status, children: game.status })
            ] }),
            expandable ? /* @__PURE__ */ u3("button", { class: "expand-btn", onClick: (e3) => {
              e3.stopPropagation();
              toggle();
            }, children: open ? "[collapse \u25B2]" : "[expand \u25BC]" }) : null
          ]
        }
      ),
      open ? /* @__PURE__ */ u3("div", { class: "row-body open", children: [
        /* @__PURE__ */ u3("div", { class: "row-details", children: [
          game.connectAddress ? /* @__PURE__ */ u3("div", { class: "connect", children: [
            "connect: ",
            /* @__PURE__ */ u3("code", { children: game.connectAddress }),
            /* @__PURE__ */ u3(
              "button",
              {
                class: "copy-btn",
                title: "copy to clipboard",
                onClick: (e3) => copyConnect(game.connectAddress, e3),
                children: "copy"
              }
            )
          ] }) : null,
          game.clientDownloadUrl ? /* @__PURE__ */ u3("div", { class: "client-link", children: /* @__PURE__ */ u3("a", { href: game.clientDownloadUrl, target: "_blank", rel: "noopener", children: "get client \u2197" }) }) : null
        ] }),
        passphrase ? /* @__PURE__ */ u3("div", { class: "admin-section", children: [
          /* @__PURE__ */ u3("div", { class: "admin-controls", children: [
            /* @__PURE__ */ u3(
              "button",
              {
                onClick: () => void handleAction("start"),
                disabled: acting || game.startBlocked,
                title: game.startBlocked ? "a conflicting game is already running on the same port" : void 0,
                children: "start"
              }
            ),
            /* @__PURE__ */ u3(
              "button",
              {
                onClick: () => void handleAction("stop"),
                disabled: acting,
                children: "stop"
              }
            ),
            /* @__PURE__ */ u3(
              "button",
              {
                type: "button",
                onClick: () => setLogsOpen((l3) => !l3),
                children: logsOpen ? "hide logs" : "logs"
              }
            )
          ] }),
          actionResult ? /* @__PURE__ */ u3("div", { class: `action-result ${actionResult.ok ? "ok" : "err"}`, children: actionResult.message }) : null
        ] }) : null,
        logsOpen && passphrase ? /* @__PURE__ */ u3("div", { class: "log-section", children: /* @__PURE__ */ u3(LogPanel, { game: id, passphrase }) }) : null
      ] }) : null
    ] });
  }

  // src/client/App.tsx
  var SESSION_KEY = "insta-game-passphrase";
  var POLL_INTERVAL_MS = 1e4;
  var PAUSE_AFTER_ACTION_MS = 15e3;
  var BACKOFF_INTERVAL_MS = 3e4;
  function App() {
    const [passphrase, setPassphrase] = d2(
      () => sessionStorage.getItem(SESSION_KEY)
    );
    const [passphraseInput, setPassphraseInput] = d2("");
    const [authError, setAuthError] = d2(false);
    const [authPending, setAuthPending] = d2(false);
    const [games, setGames] = d2(null);
    const pauseUntilRef = A2(0);
    const nextIntervalRef = A2(POLL_INTERVAL_MS);
    y2(() => {
      const stored = sessionStorage.getItem(SESSION_KEY);
      if (!stored) return;
      validatePassphrase(stored).then((ok) => {
        if (!ok) {
          sessionStorage.removeItem(SESSION_KEY);
          setPassphrase(null);
        }
      });
    }, []);
    y2(() => {
      let cancelled = false;
      async function poll() {
        if (cancelled) return;
        if (Date.now() < pauseUntilRef.current) {
          setTimeout(poll, Math.max(1e3, pauseUntilRef.current - Date.now()));
          return;
        }
        try {
          const data = await fetchStatus();
          if (!cancelled) {
            setGames(data);
            nextIntervalRef.current = POLL_INTERVAL_MS;
          }
        } catch (err) {
          const status = err.status;
          if (status === 429) nextIntervalRef.current = BACKOFF_INTERVAL_MS;
        }
        if (!cancelled) setTimeout(poll, nextIntervalRef.current);
      }
      void poll();
      return () => {
        cancelled = true;
      };
    }, []);
    const handleAuth = q2(async (e3) => {
      e3.preventDefault();
      if (!passphraseInput) return;
      setAuthPending(true);
      setAuthError(false);
      const ok = await validatePassphrase(passphraseInput);
      setAuthPending(false);
      if (!ok) {
        setAuthError(true);
        return;
      }
      sessionStorage.setItem(SESSION_KEY, passphraseInput);
      setPassphrase(passphraseInput);
      setPassphraseInput("");
    }, [passphraseInput]);
    const pausePolling = q2(() => {
      pauseUntilRef.current = Date.now() + PAUSE_AFTER_ACTION_MS;
    }, []);
    const sortedGames = games ? Object.entries(games).sort(
      ([, a3], [, b2]) => (a3.displayName || "").localeCompare(b2.displayName || "")
    ) : null;
    return /* @__PURE__ */ u3("div", { children: [
      /* @__PURE__ */ u3("div", { class: "title-bar", children: [
        /* @__PURE__ */ u3("h1", { children: "insta-game" }),
        passphrase === null ? /* @__PURE__ */ u3("form", { id: "auth-form", onSubmit: handleAuth, children: [
          /* @__PURE__ */ u3(
            "input",
            {
              type: "text",
              value: passphraseInput,
              onInput: (e3) => {
                setPassphraseInput(e3.target.value);
                setAuthError(false);
              },
              placeholder: "passphrase",
              autocomplete: "off",
              spellcheck: false,
              style: authError ? "border-color: #f44; letter-spacing: 0.15em;" : "letter-spacing: 0.15em;"
            }
          ),
          /* @__PURE__ */ u3("button", { type: "submit", disabled: authPending, children: authPending ? "checking..." : "unlock" })
        ] }) : /* @__PURE__ */ u3("span", { id: "auth-status", children: "admin" })
      ] }),
      /* @__PURE__ */ u3("div", { class: "accordion", children: sortedGames === null ? /* @__PURE__ */ u3("div", { style: "color: #666; font-size: 0.85rem;", children: "loading..." }) : sortedGames.length === 0 ? /* @__PURE__ */ u3("div", { style: "color: #666; font-size: 0.85rem;", children: "no games configured" }) : sortedGames.map(([id, game]) => /* @__PURE__ */ u3(
        GameRow,
        {
          id,
          game,
          passphrase,
          onAction: pausePolling
        },
        id
      )) })
    ] });
  }

  // src/client/index.tsx
  var root = document.getElementById("app");
  if (root) R(/* @__PURE__ */ u3(App, {}), root);
})();
