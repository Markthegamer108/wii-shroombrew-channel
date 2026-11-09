// Controller unit tests with simulated SDK objects, not console installation tests.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(require('node:path').join(__dirname, 'native-download.js'), 'utf8');

function run({destination='nand', sdResults=[], purchase={status:0,errCode:0,totalSize:42}, busy=[], launch={status:0,errCode:0}}={}) {
 const nodes={}, timers=[], calls=[];
 function element(id) {
  return nodes[id] || (nodes[id]={style:{},children:[],get firstChild(){return this.children[0];},
   appendChild(n){this.children.push(n);if(n.id)nodes[n.id]=n;},
   removeChild(n){this.children.splice(this.children.indexOf(n),1);},getElementsByTagName(){return [];}});
 }
 const context={document:{getElementById:element,createElement(){return element('created'+Object.keys(nodes).length);},
  createTextNode(value){return {value};},body:element('body')},
  setButtonText(){},location:{search:''},setTimeout(fn){timers.push(fn);},
  wiiShop:function(){this.endWaiting=function(){};this.setSCARank=function(){calls.push('mario');};},
  ECommerceInterface:function(){Object.assign(this,{
   setWebSvcUrls(){},setContentUrls(){},setParameter(){},
   checkRegistration(){return {status:0};},getDeviceInfo(){return {registrationStatus:'R',deviceToken:'token',accountId:'account'};},
   syncRegistration(){return {status:0};},
   purchaseTitle(){calls.push('purchase');return purchase;},getProgress(){return busy.shift();},
   launchTitle(title,ticket){calls.push('launch');assert.equal(title,'0001000154455354');assert.equal(ticket,'0');return launch;}
  });},
  wiiSDCard:function(){Object.assign(this,{
   isInserted(){return true;},setJournalFlag(){calls.push('journal');return 0;},
   hasProgressFinished(){calls.push('sd-poll');return sdResults.shift();},
   backupToSDCard(){calls.push('backup');return 0;},setCancelJournal(){calls.push('cleanup');return 0;}
  });},ECPrice:function(){},ECAccountPayment:function(){},ECTitleLimits:function(){}};
 vm.createContext(context);vm.runInContext(source,context);
 context.ShroombrewNativeDownload({Destination:destination,TitleID:'0001000154455354',ServiceDomain:'wiirw.bot.nu',Back:'/native-ec/'});
 nodes.underlinkR.onclick();
 let count=0;while(timers.length){assert.ok(++count<30,'controller did not settle');timers.shift()();}
 return {calls,status:nodes.nativeStatus.children.map(n=>n.value).join(''),nodes};
}
test('NAND reports completion after native EC success',()=>{
 const r=run();assert.equal(r.status,'Download complete.');assert.deepEqual(r.calls,['mario','purchase']);
});
test('native EC busy waits for the actual completion',()=>{
 const r=run({purchase:{status:-4009,downloadedSize:10,totalSize:42},busy:[{status:0,errCode:0,totalSize:42}]});
 assert.equal(r.status,'Download complete.');
});
test('native ticket failure cannot become success',()=>{
 const r=run({purchase:{status:-4035,errCode:-4035}});assert.match(r.status,/failed/);assert.doesNotMatch(r.status,/complete/);
});
test('SD numeric pending and zero success preserve the operation order',()=>{
 const r=run({destination:'sd',sdResults:[1,0,1,0,1,0]});
 assert.equal(r.status,'Download complete.');assert.equal(r.calls.filter(x=>x==='sd-poll').length,6);
 assert.ok(r.calls.indexOf('purchase')>r.calls.indexOf('journal'));
 assert.ok(r.calls.indexOf('backup')>r.calls.indexOf('purchase'));
});
test('negative SD status fails before native purchase',()=>{
 const r=run({destination:'sd',sdResults:[-22]});assert.match(r.status,/SD operation failed: -22/);assert.ok(!r.calls.includes('purchase'));
});
test('invalid SD status fails instead of reporting completion',()=>{
 const r=run({destination:'sd',sdResults:[undefined]});assert.equal(r.status,'Invalid SD operation status.');assert.ok(!r.calls.includes('purchase'));
});
test('direct app installation launches the native SD writer automatically',()=>{
 const r=run({destination:'apps'});assert.deepEqual(r.calls,['mario','purchase','launch']);
 assert.equal(r.status,'Starting SD installation...');assert.doesNotMatch(r.status,/complete|code/i);
});
test('a native installer launch failure is reported without claiming completion',()=>{
 const r=run({destination:'apps',launch:{status:-1,errCode:-1}});assert.match(r.status,/failed/);assert.doesNotMatch(r.status,/complete/);
});
