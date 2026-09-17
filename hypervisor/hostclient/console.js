import RFB from './vendor/novnc/core/rfb.js';
window.vmAlphaVNC = (element,guest) => {
 const channel=cockpit.channel({payload:'stream',spawn:['sudo','-n','/usr/libexec/vmalpha-vnc',guest],binary:true});
 const ws={readyState:0,protocol:'binary',binaryType:'arraybuffer',bufferedAmount:0,onopen:null,onmessage:null,onclose:null,onerror:null,send(b){channel.send(new Uint8Array(b.buffer||b,b.byteOffset||0,b.byteLength));},close(){channel.close();}};
 channel.addEventListener('control',(_,options)=>{if(options.command==='ready'){ws.readyState=1;ws.onopen?.({});}});
 channel.addEventListener('message',(_,b)=>{if(ws.readyState===0){ws.readyState=1;ws.onopen?.({});}const a=new Uint8Array(b);ws.onmessage?.({data:a.buffer});});
 channel.addEventListener('close',(_,options)=>{ws.readyState=3;ws.onclose?.({code:options.problem?1006:1000,reason:options.problem||'',wasClean:!options.problem});});
 const rfb=new RFB(element,ws);rfb.scaleViewport=true;rfb.resizeSession=false;
 rfb.addEventListener('disconnect',e=>{element.dataset.connection=e.detail.clean?'closed':'failed';});
 return rfb;
};
