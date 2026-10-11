(() => {
  if (window.__scuXHRProbeInstalled) return 'already installed';
  window.__scuXHRProbeInstalled = true;
  const open = XMLHttpRequest.prototype.open;
  const send = XMLHttpRequest.prototype.send;
  const key = 'scuProbeXHR';
  const save = record => {
    const records = JSON.parse(sessionStorage.getItem(key) || '[]');
    const index = records.findIndex(x => x.id === record.id);
    if (index < 0) records.push(record); else records[index] = record;
    sessionStorage.setItem(key, JSON.stringify(records));
  };
  XMLHttpRequest.prototype.open = function(method, url, ...rest) {
    this.__scuProbe = {id: Date.now() + '-' + Math.random(), method, url};
    return open.call(this, method, url, ...rest);
  };
  XMLHttpRequest.prototype.send = function(body) {
    const record = this.__scuProbe;
    if (record && /CloudPrint\/Upload/.test(record.url)) {
      record.fields = body instanceof FormData ? Array.from(body.entries()).map(([k,v]) => [k, v instanceof File ? {name:v.name,size:v.size,type:v.type} : v]) : [];
      save(record);
      this.addEventListener('loadend', () => {
        record.status = this.status;
        record.finalURL = this.responseURL;
        record.completed = true;
        save(record);
      });
    }
    return send.call(this, body);
  };
  return 'installed';
})()
