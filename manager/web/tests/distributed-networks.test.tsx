import {test} from 'node:test';
import assert from 'node:assert/strict';
import {commonEligibleUplinks} from '../src/components/DistributedNetworks';
const link=(name:string,eligible=true)=>({name,kind:'physical',mtu:1500,active:false,eligible_uplink:eligible,protection:''});
test('only common eligible uplinks across selected hosts are choices',()=>{
 const data={a:{interfaces:[link('ens224'),link('ens192',false)]},b:{interfaces:[link('ens224'),link('ens192')]},c:{interfaces:[link('other')]}};
 assert.deepEqual(commonEligibleUplinks(['a','b'],data).map(l=>l.name),['ens224']);
 assert.deepEqual(commonEligibleUplinks(['a','b','c'],data),[]);
});
test('missing host evidence never yields an eligible choice',()=>{
 assert.deepEqual(commonEligibleUplinks(['a','b'],{a:{interfaces:[link('ens224')]},b:{error:'unavailable'}}),[]);
 assert.deepEqual(commonEligibleUplinks([],{}),[]);
});
