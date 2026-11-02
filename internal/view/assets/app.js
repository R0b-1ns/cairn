// cairn view: filters, sort, count and Markdown download over prerendered rows.
// Note text is untrusted: this script only reads dataset/textContent and sets
// hidden, textContent and attributes. It never parses HTML.
(function () {
  "use strict";

  // matches reports whether a row passes the filters.
  // row: {status, task, taskBase, repo, agent, branches, authors, at, text, closed}, where branches and
  // authors are arrays: for a note, those of the note and its addenda; for an entry, its holder's.
  // f: {text, status, task, taskIsBase, repo, child, agent, branch, author, from, to, showClosed}.
  // task, repo, agent, branch and author use "*" for any (a task may legitimately be "").
  // An author is matched against names and emails, as `cairn list --author` does.
  // A base task (taskIsBase) selects itself and its extensions; any other task selects exactly itself.
  function matches(row, f) {
    if (row.closed && !f.showClosed) return false;
    if (f.status && row.status !== f.status) return false;
    if (f.task !== "*" && (f.taskIsBase ? row.taskBase !== f.task : row.task !== f.task)) return false;
    if (f.repo !== "*" && row.repo !== f.repo) return false;
    if (f.child && f.child !== "*" && row.child !== f.child) return false;
    if (f.agent !== "*" && row.agent !== f.agent) return false;
    if (f.branch && f.branch !== "*" && (row.branches || []).indexOf(f.branch) < 0) return false;
    if (f.author && f.author !== "*" && (row.authors || []).indexOf(f.author) < 0) return false;
    var day = (row.at || "").slice(0, 10);
    if (f.from && day < f.from) return false;
    if (f.to && day > f.to) return false;
    var words = (f.text || "").toLowerCase().split(/\s+/);
    for (var i = 0; i < words.length; i++) {
      if (words[i] && (row.text || "").indexOf(words[i]) < 0) return false;
    }
    return true;
  }

  // notesMarkdown is the header followed by each note's Markdown, in the given order.
  function notesMarkdown(data, ids) {
    var out = data.header || "";
    for (var i = 0; i < ids.length; i++) {
      var md = data.notes[ids[i]];
      if (typeof md === "string") out += md;
    }
    return out;
  }

  // entriesMarkdown is "# <title>", then a Markdown list: - `e-id` [by] text (note n-id), then
  // "  (superseded by id)" or "  (resolved by id)" for a closed entry, as in `cairn list`.
  // With no entry it says "No matching <noun>.".
  function entriesMarkdown(data, ids, title, noun) {
    var out = "# " + title + "\n\n";
    var listed = 0;
    for (var i = 0; i < ids.length; i++) {
      var e = data.entries[ids[i]];
      if (!e) continue;
      listed++;
      var text = String(e.text).replace(/\s*\n\s*/g, " ");
      out += "- `" + ids[i] + "` " + (e.by ? "[" + e.by + "] " : "") + text + " (note " + e.note + (e.child ? ", child " + e.child : "") + ")" +
        (e.closed ? "  (" + e.closed + ")" : "") + "\n";
    }
    if (listed === 0) out += "No matching " + noun + ".\n";
    return out;
  }

  // compare orders two sort values, numerically when asked.
  function compare(a, b, numeric) {
    if (numeric) return (parseInt(a, 10) || 0) - (parseInt(b, 10) || 0);
    a = String(a).toLowerCase();
    b = String(b).toLowerCase();
    return a < b ? -1 : a > b ? 1 : 0;
  }

  var api = { matches: matches, notesMarkdown: notesMarkdown, entriesMarkdown: entriesMarkdown, compare: compare };
  if (typeof module === "object" && module && module.exports) {
    module.exports = api;
    return;
  }
  if (typeof document === "undefined") return;

  var table = document.getElementById("rows");
  var form = document.getElementById("filters");
  var count = document.getElementById("count");
  var noMatch = document.getElementById("no-match");
  var allClosed = document.getElementById("all-closed");
  var download = document.getElementById("download-view");
  var isNotes = document.body.getAttribute("data-view") === "notes";
  var rows = table ? Array.prototype.slice.call(table.tBodies[0].rows) : [];

  function value(id, fallback) {
    var el = document.getElementById(id);
    return el ? el.value : fallback;
  }

  function current() {
    var closed = document.getElementById("show-closed");
    var task = document.getElementById("f-task");
    var picked = task && task.selectedIndex >= 0 ? task.options[task.selectedIndex] : null;
    return {
      taskIsBase: picked !== null && picked.getAttribute("data-base") === "true",
      text: value("f-text", ""),
      status: value("f-status", ""),
      task: value("f-task", "*"),
      repo: value("f-repo", "*"),
      child: value("f-child", "*"),
      agent: value("f-agent", "*"),
      branch: value("f-branch", "*"),
      author: value("f-author", "*"),
      from: value("f-from", ""),
      to: value("f-to", ""),
      showClosed: closed ? closed.checked : true
    };
  }

  function lines(s) {
    return s ? s.split("\n") : [];
  }

  function rowData(tr) {
    var d = tr.dataset;
    return {
      branches: lines(d.branches), authors: lines(d.authors),
      status: d.status || "", task: d.task || "", taskBase: d.taskBase || "", repo: d.repo || "", child: d.child || "", agent: d.agent || "",
      at: d.at || "", text: d.text || "", closed: d.closed === "true"
    };
  }

  function apply() {
    var f = current();
    var shown = 0;
    for (var i = 0; i < rows.length; i++) {
      var ok = matches(rowData(rows[i]), f);
      rows[i].hidden = !ok;
      if (ok) shown++;
    }
    if (count) count.textContent = shown + " of " + rows.length;
    // allClosed is rendered only when every row is closed; it explains an empty table better than no-match.
    if (allClosed) allClosed.hidden = f.showClosed;
    if (noMatch) noMatch.hidden = !(rows.length > 0 && shown === 0) || (allClosed !== null && !allClosed.hidden);
  }

  function visibleIds() {
    var ids = [];
    var body = table ? table.tBodies[0].rows : [];
    for (var i = 0; i < body.length; i++) {
      if (!body[i].hidden) ids.push(body[i].dataset.id);
    }
    return ids;
  }

  function save(name, text) {
    var blob = new Blob([text], { type: "text/markdown;charset=utf-8" });
    var url = URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.hidden = true;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  }

  function sortBy(th) {
    var key = th.dataset.sort;
    var numeric = th.dataset.numeric === "true";
    var dir = th.getAttribute("aria-sort") === "ascending" ? "descending"
      : th.getAttribute("aria-sort") === "descending" ? "ascending"
      : key === "at" ? "descending" : "ascending";
    var heads = table.tHead.rows[0].cells;
    for (var i = 0; i < heads.length; i++) heads[i].removeAttribute("aria-sort");
    th.setAttribute("aria-sort", dir);
    var sign = dir === "ascending" ? 1 : -1;
    var indexed = rows.map(function (tr, i) { return { tr: tr, i: i }; });
    indexed.sort(function (a, b) {
      var c = compare(a.tr.dataset[key] || "", b.tr.dataset[key] || "", numeric);
      return c !== 0 ? sign * c : a.i - b.i;
    });
    var body = table.tBodies[0];
    rows = indexed.map(function (x) { body.appendChild(x.tr); return x.tr; });
  }

  if (table && table.tHead) {
    var heads = table.tHead.rows[0].cells;
    for (var i = 0; i < heads.length; i++) {
      (function (th) {
        var b = th.querySelector("button");
        if (b && th.dataset.sort) b.addEventListener("click", function () { sortBy(th); });
      })(heads[i]);
    }
  }
  if (form) {
    form.addEventListener("input", apply);
    form.addEventListener("change", apply);
    form.addEventListener("submit", function (ev) { ev.preventDefault(); });
    form.addEventListener("reset", function () { setTimeout(apply, 0); });
  }
  // A browser ignores the download attribute of a file:// link and shows the .md instead,
  // so build the same file (header + note) from data.js; the link stays the fallback.
  var noteLink = document.getElementById("download-note");
  if (noteLink) {
    noteLink.addEventListener("click", function (ev) {
      var data = window.CAIRN;
      var id = noteLink.dataset.note;
      if (!data || !data.notes || typeof data.notes[id] !== "string") return;
      ev.preventDefault();
      save(id + ".md", notesMarkdown(data, [id]));
    });
  }
  if (download) {
    download.addEventListener("click", function () {
      var data = window.CAIRN || { header: "", notes: {}, entries: {} };
      var ids = visibleIds();
      var text = isNotes ? notesMarkdown(data, ids)
        : entriesMarkdown(data, ids, download.dataset.title || "", download.dataset.noun || "entries");
      save(download.dataset.file || "cairn.md", text);
    });
  }
  apply();
})();
