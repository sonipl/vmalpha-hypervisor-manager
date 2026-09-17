import React from 'react';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {renderToStaticMarkup} from 'react-dom/server';
import {QueryClient,QueryClientProvider} from '@tanstack/react-query';
import {MemoryRouter} from 'react-router-dom';
import {AuthProvider} from '../src/context/AuthContext';
import NativeHostsPage from '../src/pages/NativeHostsPage';
function setup(role='Platform Admin',task:unknown=null){
 const store=new Map<string,string>([['nova_user',JSON.stringify({id:'test-admin',role})],['nova_token','unit-test-only']]);
 if(task)store.set('vmalpha_native_task_test-admin',JSON.stringify(task));
 Object.defineProperty(globalThis,'localStorage',{configurable:true,value:{getItem:(key:string)=>store.get(key)||null,setItem:(key:string,v:string)=>store.set(key,v),removeItem:(key:string)=>store.delete(key)}});
 const client=new QueryClient({defaultOptions:{queries:{retry:false,gcTime:Infinity}}});
 client.setQueryData(['native-hosts'],['lab']);
 client.setQueryData(['native-inventory','lab'],{hostname:'Observed host',version:'1.0',cpu:4,cpuUsage:1,memory:{total:1024**3,used:0},kvm:true,maintenance:false,vms:[{name:'Observed guest',uuid:'vm-id',state:'shut off',cpu:1,memory:512}],pools:[{name:'default',type:'dir'}],networks:[{name:'default'}]});
 const render=()=>renderToStaticMarkup(<QueryClientProvider client={client}><MemoryRouter><AuthProvider><NativeHostsPage/></AuthProvider></MemoryRouter></QueryClientProvider>);
 return {client,render};
}
test('native page displays actual host and VM inventory',()=>{const {client,render}=setup();const html=render();assert.match(html,/Observed host/);assert.match(html,/Observed guest/);assert.match(html,/>Start</);client.clear();});
test('uncertain operation survives reload and blocks a new mutation',()=>{const {client,render}=setup('Platform Admin',{id:'previous-task',status:'Unknown',host:'lab',operation:'vm-action'});const html=render();assert.match(html,/previous-task/);assert.match(html,/<button[^>]*disabled=""[^>]*>Start<\/button>/);assert.match(html,/<button[^>]*disabled=""[^>]*>Import VM<\/button>/);client.clear();});
test('native page hides stale inventory after failed refresh',()=>{const {client,render}=setup();client.getQueryCache().find({queryKey:['native-inventory','lab']})!.setState({status:'error',error:new Error('offline')});const html=render();assert.match(html,/Live inventory is unavailable/);assert.ok(!html.includes('Observed guest'));client.clear();});
test('native page does not expose controls to non-admin users',()=>{const {client,render}=setup('Viewer');const html=render();assert.match(html,/requires Platform Admin/);assert.ok(!html.includes('Observed guest'));client.clear();});
