/* Native EC controller, based on the documented OSC/WiiMart flow.
 * No browser wallet, pairing code, ZIP job or simulated completion is used.
 * The native Shop engine owns Mario during ec.purchaseTitle(..., true).
 */
function ShroombrewNativeDownload(d) {
 var shop=null,ec=null,sd=null,started=false,journal=false,copying=false,phase='',lastBytes=0,lastTotal=0,animation=null;
 function node(id){return document.getElementById(id);}
  function L(s){return (d.D&&d.D[s])||s;}
 function text(id,value){var n=node(id);if(n){var a=n.getElementsByTagName('*');if(a.length)n=a[a.length-1];while(n.firstChild)n.removeChild(n.firstChild);n.appendChild(document.createTextNode(String(value)));}}
 function hide(id){var n=node(id);if(n)n.style.display='none';}
 function button(side,label,fn){var n=node('underButton'+side);if(!n)return;n.style.display='';setButtonText('underword'+side,label);var a=node('underlink'+side);a.href='#';a.onclick=function(){fn();return false;};}
 function line(message){text('nativeStatus',message);}
 function progress(done,total){lastBytes=done;lastTotal=total;var fill=node('nativeFill');if(total>0){fill.style.width=Math.min(100,Math.floor(100*done/total))+'%';text('nativeBytes',done+' / '+total+' bytes');if(animation)animation.setProgress(done/total);}else text('nativeBytes',L('Waiting for native EC progress...'));}
 function stopAnimation(){if(animation){try{animation.stop();}catch(e){}animation=null;}}
 function fail(message){
  copying=false;stopAnimation();line(message);hide('underButtonR');
  if(journal&&sd){try{sd.setCancelJournal();journal=false;}catch(e){text('nativeBytes','SD journal cleanup failed. '+String(e));}}
  button('L',L('Back'),function(){location.href=d.Back;});
 }
 hide('details');hide('text05-01');hide('point');hide('Wiipoint');
 // Keep the progress text below the animation band (the shop's own scene
 // occupies roughly y=170..302), where the original page puts progressBar.
 var status=document.createElement('div');status.id='nativeStatus';status.style.cssText='position:absolute;left:54px;top:306px;width:500px;text-align:center;font-size:20px';document.body.appendChild(status);
 var meter=document.createElement('div');meter.id='nativeMeter';meter.style.cssText='position:absolute;left:64px;top:332px;width:480px;height:14px;border:1px solid #34beed';meter.innerHTML='<div id="nativeFill" style="height:14px;width:0;background:#34beed"></div>';document.body.appendChild(meter);
 var bytes=document.createElement('div');bytes.id='nativeBytes';bytes.style.cssText='position:absolute;left:54px;top:352px;width:500px;text-align:center;font-size:16px';document.body.appendChild(bytes);
 button('L',L('Back'),function(){if(!started)location.href=d.Back;});
 try{
  if(typeof wiiShop==='undefined'||typeof ECommerceInterface==='undefined')throw new Error('Open this build in the native Wii Shop Channel.');
  shop=new wiiShop();var connecting=shop.connecting;shop.endWaiting();
  ec=new ECommerceInterface();
  var host=d.ServiceDomain;
  ec.setWebSvcUrls('https://ecs.'+host+'/ecs/services/ECommerceSOAP','https://ias.'+host+'/ias/services/IdentityAuthenticationSOAP','https://cas.'+host+'/cas/services/CatalogingSOAP');
  ec.setContentUrls('https://ccs.'+host+'/ccs/download','https://ccs.'+host+'/ccs/download');
  if(d.Destination==='sd'){
   if(typeof wiiSDCard==='undefined')throw new Error(L('Native SD interface unavailable.'));
   sd=new wiiSDCard();
   if(!sd.isInserted())throw new Error(L('Insert an SD card before downloading.'));
  }
  line(L(d.Destination==='apps'?'Install this app to SD Card?':(d.Destination==='sd'?'Download to SD Card?':'Download to Wii System Memory?')));
  text('nativeBytes',L('Native channel installation. Do not turn off the console.'));
  button('R',L('Download'),begin);
 }catch(e){fail(String(e));}
 function operation(p,done){
  var status=Number(p.status);
  if(status>=0){done(p);return;}
  if(status!==-4009){fail(L(phase)+' failed: '+status+' (EC '+p.errCode+')');return;}
  if(phase==='Downloading software')progress(Number(p.downloadedSize)||0,Number(p.totalSize)||0);
  setTimeout(function(){try{operation(ec.getProgress(),done);}catch(e){fail(String(e));}},500);
 }
 function call(name,fn,done){phase=name;line(L(name)+'...');try{operation(fn(),done);}catch(e){fail(L(name)+': '+String(e));}}
 function begin(){
  if(started)return;started=true;hide('underButtonR');hide('underButtonL');
  call('Checking registration',function(){return ec.checkRegistration();},function(){
   var info=ec.getDeviceInfo();
   if(info.registrationStatus==='U')call('Registering console',function(){return ec.register('NintyWhyPls');},prepare);
   else if(info.registrationStatus==='R'){
    // A console that lost its ec.cfg can still report R from the server while
    // holding no device token. Registering again is idempotent: the server
    // returns the account and token it already has for this console.
    if(info.deviceToken&&info.accountId)call('Syncing registration',function(){return ec.syncRegistration('NintyWhyPls');},prepare);
    else call('Restoring registration',function(){return ec.register('NintyWhyPls');},prepare);
   }
   else fail('Unknown native registration status: '+info.registrationStatus);
  });
 }
 function pollSD(done){
  try{
   // Archived B_10 checkSDLibProgress uses numeric results, despite the
   // community documentation describing this method as a boolean.
   var result=Number(sd.hasProgressFinished());
   if(result===0){done();return;}
   if(result<0){fail('SD operation failed: '+result);return;}
   if(!isFinite(result)){fail('Invalid SD operation status.');return;}
   setTimeout(function(){pollSD(done);},500);
  }catch(e){fail('SD operation: '+String(e));}
 }
 function prepare(){
  if(d.Destination==='sd'){
   try{
    if(!sd.isInserted()){fail(L('The SD card was removed.'));return;}
    ec.setParameter('SPACE_CHECK_POLICY','SPACE_CHECK_ENTIRE_FS');
    var ret=sd.setJournalFlag(d.TitleID);
    if(ret!==0){fail('SD journal setup failed: '+ret);return;}
    journal=true;line(L('Preparing SD Card...'));pollSD(download);
   }catch(e){fail(String(e));}
  }else download();
 }
 function download(){
  // setSCARank (OSC's Wii Shop Channel documentation):
  //   0 = small/big Mario collecting coins
  //   1 = Star/Invincible Mario
  //   2 = Star Mario running through Bullet Bills (Gold Ambassador)
  //   3 = Fire Mario (A button shoots fireballs)
  // One scene is picked for each download; ?anim=N forces one for testing.
  var scarank = parseInt((location.search.match(/[?&]anim=(\d)/)||[])[1],10);
  if(!(scarank>=0&&scarank<=3))scarank = Math.floor(Math.random()*4);
  try{shop.setSCARank(scarank);}catch(e){}
  // On the console the Shop itself draws Mario during purchaseTitle; a DOM
  // copy would draw a second Mario on top, so only the browser flow uses it.
  if(!(typeof shroomNativeSound!=='undefined'&&shroomNativeSound)&&!animation&&typeof ShroomMarioDownload==='function'){try{animation=new ShroomMarioDownload(document.body, scarank);}catch(e){animation=null;}}
  call('Downloading software',function(){return ec.purchaseTitle(d.TitleID,'0',new ECPrice('0.00','POINTS'),new ECAccountPayment(),new ECTitleLimits(),true,'','','0');},function(p){
   if(Number(p.errCode)<0){fail('Native download failed: '+p.errCode);return;}
   progress(lastTotal||Number(p.totalSize)||lastBytes,lastTotal||Number(p.totalSize)||lastBytes);
   if(d.Destination==='sd')transfer();else complete();
  });
 }
 function transfer(){
  stopAnimation();
  try{
   ec.setParameter('SPACE_CHECK_POLICY','SPACE_CHECK_USER_ONLY');
   line(L('Transferring software to SD Card...'));text('nativeBytes',L('Do not remove the SD card.'));
   // The SD interface exposes completion, not a byte-progress counter.
   // Leave the native download bar full; do not invent transfer percentages.
   copying=true;var ret=sd.backupToSDCard(d.TitleID);
   if(ret!==0){fail('SD transfer failed: '+ret);return;}
   pollSD(function(){copying=false;var r=sd.setCancelJournal();if(r!==0){fail('SD journal cleanup failed: '+r);return;}journal=false;pollSD(complete);});
  }catch(e){fail('SD transfer: '+String(e));}
 }
 function complete(){
  stopAnimation();
  if(d.Destination==='apps'){
   // The native installer continues automatically and writes ordinary files
   // into sd:/apps. Launch success alone is not SD installation completion.
   text('nativeBytes',L('Starting the SD installer. Keep the SD card inserted.'));
   call('Starting SD installer',function(){return ec.launchTitle(d.TitleID,'0');},function(){line(L('Starting SD installation...'));});
   return;
  }
  line(L('Download complete.'));text('nativeBytes',L(d.Destination==='sd'?'Installed to the SD Card Menu.':'Installed to Wii System Memory.'));
  node('nativeFill').style.width='100%';
  button('R','OK',function(){location.href=d.Back;});
 }
}
