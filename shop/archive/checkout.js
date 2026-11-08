/* Shroombrew account adapter for original captured B_08/B_09/B_10/P_01 markup. ES3. */
(function () {
 var d=archiveShop;
  // Translations: d.D maps the English text used on these pages to the console's language.
  function L(s,v){var r=(d.D&&d.D[s])||s;if(v)for(var k in v)if(Object.prototype.hasOwnProperty.call(v,k))r=r.split('{'+k+'}').join(String(v[k]));return r;}
  function translateStatic(n){
   var c=n.childNodes,i;
   for(i=0;i<c.length;i++){
    if(c[i].nodeType===3){var raw=c[i].nodeValue,tr=raw.replace(/^\s+|\s+$/g,'');if(tr&&d.D&&d.D[tr]){var m=raw.match(/^(\s*)[\s\S]*?(\s*)$/);c[i].nodeValue=m[1]+d.D[tr]+m[2];}}
    else if(c[i].nodeType===1&&c[i].tagName!=='SCRIPT'&&c[i].tagName!=='STYLE')translateStatic(c[i]);
   }
  }
 if(d.Mode==='native'){try{translateStatic(document.body);}catch(e){}ShroombrewNativeDownload(d);return;}
 function el(id){return document.getElementById(id);}
 function text(id,value){var n=el(id);if(n){var children=n.getElementsByTagName('*');if(children.length)n=children[children.length-1];while(n.firstChild)n.removeChild(n.firstChild);n.appendChild(document.createTextNode(String(value)));}}
 function show(id,on){var n=el(id);if(n)n.style.display=on?'':'none';}
 function sound(hover){try{if(window.snd&&snd.playSE)snd.playSE(hover?cSE_Forcus:cSE_Decide);}catch(e){}}
 function go(url){sound(false);if(parent!==window&&typeof parent.shroomGoto==='function')parent.shroomGoto(url);else if(parent!==window&&typeof parent['goto']==='function')parent['goto'](url.charAt(0)==='/'?location.origin+url:url);else location.href=url;}
 function bind(id,fn){var n=el(id);if(n){n.onclick=function(){fn();return false;};}}
 function button(side,label,fn){show('underButton'+side,true);setButtonText('underword'+side,label);var a=el('underlink'+side);if(a){a.href='#';a.onclick=function(){sound(false);fn();return false;};a.onmouseover=function(){var i=el('underImg'+side);if(i)i.src='/archive-shop/assets/images/banner/under_banner_b.gif';sound(true);};a.onmouseout=function(){var i=el('underImg'+side);if(i)i.src='/archive-shop/assets/images/banner/under_banner_a.gif';};}}
 var submitting=false;
 function submit(){if(submitting)return;submitting=true;el('archiveForm').submit();}
 function points(){go('/points?return='+encodeURIComponent(d.Next||d.Back));}
 try{translateStatic(document.body);}catch(e){}
  bind('top',function(){go('/oss/serv/W_03.html');});
 bind('help',function(){go('/help');});
 bind('balanceInfo',points);
 for(var si=0;si<3;si++){(function(id){var n=el(id);if(n)n.onmouseover=function(){sound(true);};})(['top','help','balanceInfo'][si]);}
 text('currentBalance',d.Balance);
 try{parent.points=d.Balance;}catch(e){}
 try{if(typeof wiiShop!=='undefined'){var shopHandle=new wiiShop();var shopConnecting=shopHandle.connecting;shopHandle.endWaiting();}}catch(e){}
 button('L',L('Back'),function(){go(d.Back);});
 var imgs=document.getElementsByTagName('img');
 for(var i=0;i<imgs.length;i++){
  if(imgs[i].src.indexOf('/spacer.gif')!==-1)imgs[i].style.cursor='pointer';
 }
 if(d.Page==='B_08'){
  text('B08headingMessage',L('Selection Confirmation'));
  var controllers=el('controller'),icons={'Wii Remote':'Rvl','Classic Controller':'Classic','Nunchuk':'Nunchuk','GameCube Controller':'Gc','USB Keyboard':'Keyboard','Wii Balance Board':'Balance'};
  if(controllers){while(controllers.firstChild)controllers.removeChild(controllers.firstChild);for(var name in icons)if(Object.prototype.hasOwnProperty.call(icons,name)&&d.Controllers.indexOf(name)!==-1){var icon=document.createElement('img');icon.src='/archive-shop/assets/images/banner/B_08_'+icons[name]+'Ctrl.gif';icon.width=68;icon.height=51;icon.alt=name;controllers.appendChild(icon);}}
  button('R','OK',function(){go(d.Next);});
 }
 if(d.Page==='B_09'){
  text('pointcost01',d.Balance);text('pointword02',d.Price);text('pointword03',d.Remaining);
  var needBlocks=Math.ceil((d.FSBlocks||0)/8);
  var openBlocks=parseInt(d.FreeBlocks,10);if(!(openBlocks>=0))openBlocks=2163;
  var afterBlocks=Math.max(openBlocks-needBlocks,0);
  text('blockcost01',openBlocks);text('blockcost02',needBlocks);text('blockcost03',afterBlocks);
  text('BuyMsg',L('Download this homebrew package for {n} Wii Points.',{n:d.Price}));
  if(d.Balance<d.Price){
   text('confirmationMsg',L('You need more Wii Points.'));
   button('R',L('Add Wii Points'),points);
  }else if(needBlocks>openBlocks){
   text('confirmationMsg',L('There is not enough space in the Wii System Memory.'));
   button('R',L('Back'),function(){go(d.Back);});
  }else{
   text('confirmationMsg',L('Download this software?'));
   button('R',L('Download'),submit);
  }
 }
 if(d.Page==='P_01'){
  var pq='?return='+encodeURIComponent(d.Next||d.Back);
  bind('card01',function(){go('/points/card'+pq);});
  bind('card02',function(){go('/points/buy'+pq);});
  for(var j=1;j<=2;j++)(function(n){var c=el('card0'+n);if(c){c.onmouseover=function(){el('card0'+n+'_Img').src='/archive-shop/assets/images/banner/banner03_b.gif';sound(true);};c.onmouseout=function(){el('card0'+n+'_Img').src='/archive-shop/assets/images/banner/banner03_a.gif';};}})(j);
 }
 if(d.Page==='P_02'){
  button('L',L('Quit'),function(){go(d.Back);});
  button('R','OK',submit);
  var num=el('cardNumber');
  if(num){
   num.onkeydown=function(e){e=e||window.event;if(e.keyCode===13){submit();return false;}};
   try{num.focus();}catch(e){}
  }
 }
 // Add Wii Points screens after P_02 (points_flow.go).
 function picks(fn){var all=document.getElementsByTagName('img');for(var i=0;i<all.length;i++){if((' '+all[i].className+' ').indexOf(' pick ')!==-1)(function(n){n.onclick=function(){fn(n);return false;};})(all[i]);}}
 function swap(trigger,image,a,b){var t=el(trigger),i=el(image);if(!t||!i)return;t.onmouseover=function(){i.src='/archive-shop/assets/images/banner/'+b;sound(true);};t.onmouseout=function(){i.src='/archive-shop/assets/images/banner/'+a;};}
 if(d.Page==='P_04'){
  button('L',L('Do Not Redeem'),function(){go(d.Back);});
  button('R',L('Redeem'),submit);
 }
 if(d.Page==='P_06'){
  picks(function(n){go(d.Next+'&points='+n.getAttribute('data-points'));});
  for(var oi=0;oi<4;oi++)swap('point0'+oi+'Pick','point0'+oi+'Img','add_points_a.gif','add_points_b.gif');
 }
 if(d.Page==='P_07'){
  picks(function(n){go(d.Next+'&card='+n.getAttribute('data-card'));});
  swap('card02Spacer','card02Img','card02_a.gif','card02_b.gif');swap('card03Spacer','card03Img','card03_a.gif','card03_b.gif');
 }
 if(d.Page==='P_08'){
  var panel='';
  var openPanel=function(id){panel=id;show(id,true);show('underButtonR',false);sound(false);};
  button('L',L('Back'),function(){if(panel){show(panel,false);panel='';show('underButtonR',true);return;}go(d.Back);});
  button('R','OK',function(){go(d.Next);});
  // The expiry date steps like the shop's arrows; nothing here is ever charged.
  var step=function(id,delta,lo,hi){var f=el(id);if(!f)return;var v=parseInt(f.value,10)+delta;if(v<lo)v=hi;if(v>hi)v=lo;f.value=String(v);sound(false);};
  var year=new Date().getFullYear();
  bind('expMonthUp',function(){step('cardExpMM',1,1,12);});bind('expMonthDown',function(){step('cardExpMM',-1,1,12);});
  bind('expYearUp',function(){step('cardExpYY',1,year,year+10);});bind('expYearDown',function(){step('cardExpYY',-1,year,year+10);});
  swap('expMonthUp','expMonthUp','arrowUp_a.gif','arrowUp_b.gif');swap('expMonthDown','expMonthDown','arrowDw_a.gif','arrowDw_b.gif');
  swap('expYearUp','expYearUp','arrowUp_a.gif','arrowUp_b.gif');swap('expYearDown','expYearDown','arrowDw_a.gif','arrowDw_b.gif');
  bind('securityCodeButton',function(){openPanel('secExplain');});bind('securityStatementButton',function(){openPanel('secStatement');});
  swap('securityCodeButton','secExplainImg','option_a.gif','option_b.gif');swap('securityStatementButton','statementImg','option_a.gif','option_b.gif');
 }
 if(d.Page==='P_12'){
  button('L',L('Yes'),submit);
  button('R',L('No'),function(){go(d.Back);});
 }
 if(d.Page==='P_15'){
  show('underButtonL',false);
  try{snd.playSE(cSE_AddPoint);}catch(e){}
  button('','OK',function(){go(d.Back);});
 }
 if(d.Flash){try{snd.playSE(d.FlashBad?cSE_Error:cSE_AddPoint);}catch(e){}}
 if(d.Page==='B_10'){
  show('details',false);show('text05-01',false);
  var status=document.createElement('div');status.id='archiveStatus';document.body.appendChild(status);
  var note=document.createElement('div');note.id='archiveNote';document.body.appendChild(note);
  status.style.top='148px';
  text('archiveNote',L('Preparing software for your SD card.'));
  var animPick=parseInt((location.search.match(/[?&]anim=(\d)/)||[])[1],10);// 0 coins, 1 Star Mario, 2 Bullet Bills, 3 Fire Mario; ?anim=N forces one.
   if(!(animPick>=0&&animPick<=5)){animPick=Math.floor(Math.random()*4);if(Math.random()<0.05)animPick=4;}var animation=new ShroomMarioDownload(document.body,animPick),finished=false;
  function readyActions(){text('archiveStatus',L('Your homebrew package is ready.'));text('archiveNote',L('Use Shroombrew Installer on your Wii, or')+' ');var zip=document.createElement('a');zip.href='#';zip.appendChild(document.createTextNode(L('download the ZIP instead.')));zip.onclick=function(){submit();return false;};el('archiveNote').appendChild(zip);button('R',L('Install on Wii'),function(){el('archiveForm').action='/shop-download/installer/'+encodeURIComponent(d.Job);submit();});}
  function finish(message,ready){if(finished)return;finished=true;text('archiveStatus',message);if(ready)animation.finish(readyActions);else animation.stop();}
  if(d.Mode==='pc'){
   // PC browser: the same Download Software screen as the console. The page's
   // own lines (You downloaded / title / points and blocks after download /
   // the big blue Download successful! / the centred OK) are filled in place.
   if(status)status.style.display='none';
   if(note)note.style.display='none';
   show('underButtonL',false);
   show('details',true);
   if(d.TitleID){try{sessionStorage.setItem('shroomInstalled_'+d.TitleID,''+ (d.TitleVersion||0));}catch(e){}}
   text('rcvHead',L('You are downloading'));
   text('pointCost',String(d.Balance||0));
   text('blockCost',String(Math.max(0,(parseInt(d.FreeBlocks,10)||2163)-Math.ceil((d.FSBlocks||0)/8))));
   var bigStatus=el('text03-01');
   function setStatus(html){if(bigStatus)bigStatus.innerHTML=html;}
   // The original keeps the status line empty until the download succeeds.
   if(bigStatus)bigStatus.style.display='none';
   // Show the centre OK the moment the download starts; it is the page's own.
   ['bannershadow','spacerL','bannerL','wordL'].forEach(function(id){var n=el(id);if(n)n.style.display='';});
   var okAnchor=document.getElementById('OKBtnAnchor');
   if(okAnchor){okAnchor.href='#';okAnchor.onclick=function(){try{wiiSelectSound();}catch(e){}go((d.Health&&d.Health!=='')?d.Health:d.Back);return false;};}
   setTimeout(function(){
     if(animation)animation.stop();
     var f=document.createElement('iframe');f.style.display='none';f.src=d.Action;document.body.appendChild(f);
     try{window.location.href=d.Action;}catch(e){}
     // The original switches the header, moves the status line down to 226px
     // and uses its blue title style for the success message.
     text('rcvHead',L('You downloaded'));
     if(bigStatus){bigStatus.style.display='';bigStatus.style.top='226px';}
     setStatus('<div align="center" class="titleBlueL">'+L('Download successful!')+'</div>');
   },1500);
   return;
  }
  if(d.Mode==='installer'){
   animation.stop();text('rcvHead',L('Install to SD Card'));text('archiveStatus',L('Your install code: {code}',{code:d.Code}));
   var codeText=document.createElement('div');codeText.style.cssText='position:absolute;left:64px;top:200px;width:480px;font-size:18px;line-height:24px;text-align:center';codeText.appendChild(document.createTextNode(L('Press HOME, open the Homebrew Channel, and launch Shroombrew Installer. Enter this code within 15 minutes. Keep your SD card inserted.')));document.body.appendChild(codeText);
   text('archiveNote',L('Need the installer?')+' ');var installer=document.createElement('a');installer.href='/tools/wii-installer.zip';installer.appendChild(document.createTextNode(L('Download Shroombrew Installer')));el('archiveNote').appendChild(installer);return;
  }
  if(d.Message){finish(L(d.Message),false);return;}
  function poll(){var x=new XMLHttpRequest();x.open('GET','/shop-download/status/'+encodeURIComponent(d.Job)+'?format=text',true);x.onreadystatechange=function(){if(x.readyState!==4)return;var a=x.responseText.split('\t');if(x.status!==200||a.length!==3){finish(L('Connection interrupted. Return and try again.'),false);return;}if(a[0]==='ready'){finish(L('Package verified. Finishing...'),true);return;}if(a[0]==='failed'){finish(L('Download unavailable. No points were charged.'),false);return;}var total=parseInt(a[2],10)||0,bytes=parseInt(a[1],10)||0;if(total>0)animation.setProgress(Math.min(.98,bytes/total));text('archiveStatus',a[0]==='extracting'?L('Verifying software...'):L('Downloading software...')+(total>0?' '+Math.floor(100*bytes/total)+'%':''));setTimeout(poll,700);};x.send(null);}
  text('archiveStatus',L('Connecting. Please wait.'));poll();
 }
})();