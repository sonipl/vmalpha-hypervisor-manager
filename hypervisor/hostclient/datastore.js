'use strict';
window.datastoreBrowser=async function(){
 const d=document.querySelector('#dialog'),root='/var/lib/libvirt/images';
 let current=root,selected=null,listing=null,working=false;
 d.oncancel=e=>{if(working)e.preventDefault();};d.classList.add('datastore-dialog');d.addEventListener('close',()=>d.classList.remove('datastore-dialog'),{once:true});
 const icon=(kind)=>`<svg class="ds-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${kind==='folder'?'M3 6h7l2 3h9v11H3z M3 6V4h7l2 2':kind==='pool'?'M4 4h16v16H4z M4 9h16 M7 6h2 M7 13h2 M7 17h2':'M6 2h8l5 5v15H6z M14 2v6h5'}"/></svg>`;
 const poolsHere=data.pools.filter(p=>p.type==='dir'&&(p.path===root||p.path.startsWith(root+'/')));
 const message=(s,error=false)=>{const e=d.querySelector('#ds-message');e.textContent=s;e.classList.toggle('error-text',error);};
 const displaySize=n=>n<1048576?(n/1024).toFixed(1)+' KiB':fmt(n);
 function details(){
  d.querySelector('#ds-details').innerHTML=selected?`<div class="file-icon">${icon(selected.directory?'folder':'file')}</div><strong>${esc(selected.name)}</strong>${dl([['Type',selected.directory?'Directory':'File'],['Size',selected.directory?'—':displaySize(selected.size)],['Modified',new Date(selected.modified*1000).toLocaleString()],['Location',selected.path]])}`:'<p>Select a file or directory to view details.</p>';
  d.querySelectorAll('[data-ds]').forEach(b=>{const op=b.dataset.ds;b.disabled=working||(['download','trash','move','copy'].includes(op)&&!selected)||(['download','copy'].includes(op)&&selected?.directory)||(op==='download'&&selected?.size>256*1024*1024);});
  d.querySelectorAll('[data-item],[data-dir],#ds-close,#ds-done').forEach(b=>b.disabled=working);
  d.querySelectorAll('[data-item]').forEach(b=>b.classList.toggle('selected',b.dataset.item===selected?.path||(b.dataset.directory&&current.startsWith(b.dataset.item))));
 }
 async function load(path=current){
  listing=await api('ds-list',{path});current=listing.path;selected=null;
  const chain=[root];for(const part of current.slice(root.length).split('/').filter(Boolean))chain.push(chain[chain.length-1]+'/'+part);
  const levels=await Promise.all(chain.map(p=>p===current?listing:api('ds-list',{path:p})));
  const allEntries=levels.flatMap(l=>l.entries);const activePool=poolsHere.filter(p=>current===p.path||current.startsWith(p.path+'/')).sort((a,b)=>b.path.length-a.path.length)[0];
  const files=listing.entries.filter(f=>!f.directory),pct=Math.round((listing.total-listing.free)/listing.total*100);
  d.innerHTML=`<div class="dialog-title">▤ Datastore browser<button id="ds-close" aria-label="Close datastore browser">×</button></div><div class="ds-toolbar">${[['upload','↑ Upload'],['download','↓ Download'],['trash','♲ Delete'],['move','Move'],['copy','Copy'],['mkdir','▱ Create directory'],['refresh','⟳ Refresh'],['trash-list','Trash']].map(([op,label])=>`<button data-ds="${op}" ${op==='download'?'title="Download files up to 256 MiB; use SSH for larger files"':''}>${label}</button>`).join('')}<div class="ds-capacity"><span>${files.length} file(s)</span><div class="bar"><span style="width:${pct}%"></span></div><b>${pct}%</b></div></div><input type="file" id="ds-upload" aria-label="Upload file" hidden><div class="ds-columns"><div class="ds-column ds-pools">${poolsHere.map(p=>`<button data-dir="${esc(p.path)}" class="${activePool?.name===p.name?'selected':''}">${icon('pool')} ${esc(p.name)}</button>`).join('')}</div><div class="ds-path-columns">${levels.map((level,i)=>`<div class="ds-column ds-level" aria-label="${esc(level.path)}">${level.entries.map(f=>`<button data-item="${esc(f.path)}" ${f.directory?'data-directory="true"':''} class="${chain[i+1]===f.path?'selected':''}" title="${esc(f.name)}">${icon(f.directory?'folder':'file')} <span class="ds-name">${esc(f.name)}</span></button>`).join('')||'<p class="muted">Empty directory</p>'}</div>`).join('')}</div><div class="ds-column ds-details" id="ds-details"></div></div><div id="ds-form"></div><div id="ds-message" role="status"></div><div class="ds-breadcrumb"><span>▤ ${esc(current)}</span><span>${displaySize(listing.free)} free / ${displaySize(listing.total)}</span></div><div class="dialog-footer"><span class="muted">Select a directory to browse. Delete moves items to recoverable trash.</span><button class="primary" id="ds-done">Close</button></div>`;
  if(!d.open)d.showModal();d.querySelector('#ds-close').onclick=d.querySelector('#ds-done').onclick=()=>{if(!working)d.close();};
  d.querySelectorAll('[data-dir]').forEach(b=>b.onclick=()=>safe(()=>load(b.dataset.dir)));
  d.querySelectorAll('[data-item]').forEach(b=>{b.onclick=()=>{if(working)return;if(b.dataset.directory){safe(()=>load(b.dataset.item));return;}selected=allEntries.find(f=>f.path===b.dataset.item);details();};});
  d.querySelectorAll('[data-ds]').forEach(b=>b.onclick=()=>safe(()=>actionDS(b.dataset.ds)));
  d.querySelector('#ds-upload').onchange=e=>{const file=e.target.files[0];if(file)safe(()=>upload(file));};details();
  if(listing.truncated)message('Showing first 2000 entries. Use subdirectories to narrow the view.');
 }
 async function safe(fn){try{await fn();}catch(e){message(e.message,true);}}
 function formDS(title,body,callback){
  d.querySelector('#ds-form').innerHTML=`<form id="ds-action-form"><strong>${esc(title)}</strong>${body}<button type="submit" class="primary">Confirm</button><button type="button" id="ds-cancel-action">Cancel</button></form>`;
  d.querySelector('#ds-cancel-action').onclick=()=>d.querySelector('#ds-form').replaceChildren();
  d.querySelector('#ds-action-form').onsubmit=e=>{e.preventDefault();if(working)return;const values=Object.fromEntries(new FormData(e.target));working=true;details();e.target.querySelectorAll('button,input').forEach(x=>x.disabled=true);safe(async()=>{try{await callback(values);await load();message('Completed successfully');}finally{working=false;details();d.querySelectorAll('#ds-action-form button,#ds-action-form input').forEach(x=>x.disabled=false);}});};
 }
 async function actionDS(op){
  const chosenFile=selected;
  if(op==='refresh')return load();
  if(op==='upload'){d.querySelector('#ds-upload').click();return;}
  if(op==='mkdir')return formDS('Create directory',`<label>Name <input name="name" required pattern="[^/]+"></label>`,a=>api('ds-mkdir',{path:current+'/'+a.name}));
  if(op==='trash')return formDS('Move '+selected.name+' to trash?',`<span>Referenced VM files are protected.</span>`,()=>api('ds-trash',{path:chosenFile.path}));
  if(op==='move'||op==='copy')return formDS((op==='move'?'Move ':'Copy ')+selected.name,`<label>Destination path <input name="destination" required value="${esc(current+'/'+selected.name+(op==='copy'?'.copy':''))}"></label>`,a=>api('ds-'+op,{path:chosenFile.path,destination:a.destination}));
  if(op==='trash-list'){
   const rows=await api('ds-trash-list');
   d.querySelector('#ds-form').innerHTML=`<div class="trash-items"><strong>Recoverable trash</strong>${rows.map(r=>`<div><span>${esc(r.path)}</span><button data-restore="${r.token}">Restore</button></div>`).join('')||'<p>Trash is empty.</p>'}</div>`;
   d.querySelectorAll('[data-restore]').forEach(b=>b.onclick=()=>safe(async()=>{await api('ds-restore',{token:b.dataset.restore});await load();message('Item restored');}));return;
  }
  if(op==='download'){
   const file=selected,parts=[];let offset=0;working=true;details();
   try{do{const r=await api('ds-read',{path:file.path,identity:file.identity,offset});const raw=atob(r.data);parts.push(Uint8Array.from(raw,c=>c.charCodeAt(0)));offset=r.offset;message('Downloading '+Math.round(offset/Math.max(file.size,1)*100)+'%');}while(offset<file.size);
    const url=URL.createObjectURL(new Blob(parts));const link=document.createElement('a');link.href=url;link.download=file.name;link.click();setTimeout(()=>URL.revokeObjectURL(url),60000);message('Download prepared');
   }finally{working=false;details();}
  }
 }
 async function upload(file){
  let token=null;working=true;details();
  try{
   token=(await api('ds-upload-begin',{path:current+'/'+file.name,size:file.size})).token;
   for(let offset=0;offset<file.size;offset+=4*1024*1024){const bytes=new Uint8Array(await file.slice(offset,offset+4*1024*1024).arrayBuffer());let binary='';for(let i=0;i<bytes.length;i+=32768)binary+=String.fromCharCode(...bytes.subarray(i,i+32768));await api('ds-upload-chunk',{token,offset,data:btoa(binary)});message('Uploading '+Math.round((offset+bytes.length)/file.size*100)+'%');}
   await api('ds-upload-finish',{token});token=null;await load();message('Upload complete');
  }finally{if(token)await api('ds-upload-cancel',{token}).catch(()=>{});working=false;details();}
 }
 await safe(()=>load());
};
