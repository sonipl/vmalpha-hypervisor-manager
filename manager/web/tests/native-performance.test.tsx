import React from 'react';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {renderToStaticMarkup} from 'react-dom/server';
import {QueryClient,QueryClientProvider} from '@tanstack/react-query';
import NativePerformance from '../src/components/NativePerformance';
function fixture(value:unknown,failed=false){const client=new QueryClient({defaultOptions:{queries:{retry:false,gcTime:Infinity}}});client.setQueryData(['native-metrics','lab','cpu','1h','host',''],value);if(failed)client.getQueryCache().find({queryKey:['native-metrics','lab','cpu','1h','host','']})!.setState({status:'error',error:new Error('offline')});const html=renderToStaticMarkup(<QueryClientProvider client={client}><NativePerformance host="lab"/></QueryClientProvider>);client.clear();return html;}
test('missing performance data is never reported as healthy',()=>{const html=fixture({available:false,series:[]});assert.match(html,/Monitoring data is unavailable/);assert.ok(!html.includes('Collector healthy'));});
test('old samples are explicitly marked stale even if collector reports healthy',()=>{const html=fixture({available:true,collectorState:'healthy',timestamp:1000,unit:'%',series:[{labels:{},values:[[100,42]]}]});assert.match(html,/samples stale/);assert.ok(!html.includes('Collector healthy'));});
test('failed refresh hides cached healthy performance result',()=>{const html=fixture({available:true,collectorState:'healthy',timestamp:1000,unit:'%',series:[{labels:{},values:[[999,42]]}]},true);assert.match(html,/Monitoring data is unavailable/);assert.ok(!html.includes('Collector healthy'));});
