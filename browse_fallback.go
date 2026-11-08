package main

// B_16 (Search by Genre) and B_17 (Search by Publisher) were captured with
// ES6 renderers: const, URLSearchParams, Array.includes and template literals.
// Wii Opera rejects the whole block, leaving those pages blank. This ES3
// renderer paints the same banner grids from gameslist, including the Demo
// genre used by WiiWare demo releases.
func classicBrowseJS() []byte {
	return []byte(`(function(){
var retries=0;
function value(name){var q=location.search.substring(1).split('&');for(var i=0;i<q.length;i++){var p=q[i].split('=');if(decodeURIComponent(p[0])==name)return decodeURIComponent((p[1]||'').replace(/\+/g,' '));}return '';}
function esc(s){return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');}
function section(){
 var c=value('category');if(c)return c;
 if(value('vc')=='true')return 'Virtual Console';
 var p=value('platform').toLowerCase();
 if(p.indexOf('wiiware')>=0)return 'WiiWare';
 if(p.indexOf('virtual')>=0)return 'Virtual Console';
 if(/channel|kanal|cha.ne|canal/.test(p))return 'Channels';
 return '';
}
var licensed={'Virtual Console':1,'WiiWare':1,'DLC':1,'Channels':1};
function inSection(game){
 var g=String(game.genre||'');
 var cat=section();
 if(value('hb')=='1'){
  if(cat=='Virtual Console')return g=='emulators';
  return g=='games'||g=='utilities'||g=='media'||g=='demos';
 }
 return !cat||g==cat;
}
function tokens(game){
 var g=String(game.genre||'');
 var out=[];
 if(licensed[g]){
  var parts=String(game.displayGenre||'').split(',');
  for(var i=0;i<parts.length;i++){var t=parts[i].replace(/^\s+|\s+$/g,'');if(t&&t!='document.write')out.push(t);}
 }else if(g&&g!='flash')out.push(g);
 return out;
}
function render(){
 var box=document.getElementById('searchItem');
 if(!box||typeof gameslist=='undefined'){if(retries++<20)setTimeout(render,250);return;}
 var page=location.pathname.indexOf('B_17')>=0?'publisher':'genre';
 var values=[],counts=[],i,j;
 function add(v){
  if(!v)return;
  var k=-1;for(var n=0;n<values.length;n++)if(values[n].toLowerCase()==v.toLowerCase()){k=n;break;}
  if(k<0){values.push(v);counts.push(1);}else counts[k]++;
 }
 for(i=1;i<gameslist.length;i++){
  var game=gameslist[i];
  if(!game||!game.id||!inSection(game))continue;
  if(page=='publisher')add(game.publisher);
  else{var ts=tokens(game);for(j=0;j<ts.length;j++)add(ts[j]);}
 }
 var order=[];
 for(i=0;i<values.length;i++)order.push(i);
 order.sort(function(a,b){return values[a].toLowerCase()<values[b].toLowerCase()?-1:1;});
 if(page=='publisher'){
  for(i=0;i<order.length;i++)if(values[order[i]]=='Nintendo'){var n=order.splice(i,1)[0];order.unshift(n);break;}
 }
 var q=location.search;
 var html='';
 for(i=0;i<order.length;i++){
  var idx=order[i],v=values[idx],count=counts[idx];
  if(page=='publisher'){
   html+='<div id="publisherBanner0'+(i+1)+'" style="position:absolute;left:'+(6+(i%2)*235)+'px;top:'+(2+Math.floor(i/2)*54)+'px;width:235px;height:55px;">'+
    '<div id="banner01" style="position:absolute;left:0;top:0;width:235px;height:53px;z-index:5;"><img src="/oss/oss/common/images//banner/B_17_publisher_a_00.png" id="Image0'+(i+1)+'" width="235" height="53" border="0" /></div>'+
    '<div id="word0'+(i+1)+'" style="position:absolute;left:6px;top:6px;width:223px;height:22px;z-index:6;overflow:hidden;" align="center"><table height="100%"><tr><td align="left" valign="middle"><span class="contentsBlack">'+esc(v)+'</span></td></tr></table></div>'+
    '<div id="hit0'+(i+1)+'-01" style="position:absolute;left:114px;top:31px;width:84px;height:16px;z-index:6;"><table width="100%" height="100%"><tr><td align="right" valign="top"><span class="contentsBlack">Titles:</span></td></tr></table></div>'+
    '<div id="hit0'+(i+1)+'-02" style="position:absolute;left:198px;top:31px;width:31px;height:16px;z-index:6;"><table width="100%" height="100%"><tr><td align="right" valign="top"><span class="contentsBlack">'+count+'</span></td></tr></table></div>'+
    '<div id="spacer0'+(i+1)+'" style="position:absolute;left:0;top:0;width:235px;height:55px;z-index:7;"><a href="javascript:showPage(\'B_04.html'+q+'&p=1&publisher='+encodeURIComponent(v)+'\')"><img src="/oss/oss/common/images//spacer.gif" name="Image0'+(i+1)+'s" width="235" height="55" border="0" id="Image0'+(i+1)+'s" onmouseover="MM_swapImage(\'Image0'+(i+1)+'\',\'\',\'/oss/oss/common/images//banner/B_17_publisher_a_01.png\',1);snd.playSE(cSE_Forcus);" onmouseout="MM_swapImgRestore();" onclick="snd.playSE(cSE_Decide);" /></a></div></div>';
  }else{
   html+='<div id="banner0'+(i+1)+'" style="position:absolute;left:'+((i%2)*244)+'px;top:'+(1+Math.floor(i/2)*92)+'px;width:244px;height:91px;z-index:5;"><img src="/oss/oss/common/images//banner/B_04_half_genre_a.png" width="244" height="91" id="Image0'+(i+1)+'" /></div>'+
    '<div id="word0'+(i+1)+'" style="position:absolute;left:'+(15+(i%2)*244)+'px;top:'+(6+Math.floor(i/2)*92)+'px;width:212px;height:52px;z-index:6;" align="center"><table height="100%"><tr><td align="left" valign="middle"><span class="catalogTitleBlack">'+esc(v)+'</span></td></tr></table></div>'+
    '<div id="hit0'+(i+1)+'-01" style="position:absolute;left:'+(114+(i%2)*244)+'px;top:'+(60+Math.floor(i/2)*92)+'px;width:84px;height:16px;z-index:6;"><table width="100%" height="100%"><tr><td align="right" valign="top"><span class="contentsBlack">Titles:</span></td></tr></table></div>'+
    '<div id="hit'+(i+1)+'-02" style="position:absolute;left:'+(198+(i%2)*244)+'px;top:'+(60+Math.floor(i/2)*92)+'px;width:27px;height:16px;z-index:6;"><table width="100%" height="100%"><tr><td align="right" valign="top"><span class="contentsBlack">'+count+'</span></td></tr></table></div>'+
    '<div id="spacer0'+(i+1)+'" style="position:absolute;left:'+((i%2)*244)+'px;top:'+(1+Math.floor(i/2)*92)+'px;width:244px;height:91px;z-index:7;"><a href="javascript:showPage(\'B_04.html'+q+'&p=1&genre='+encodeURIComponent(v)+'\')"><img src="/oss/oss/common/images//spacer.gif" name="Image0'+(i+1)+'s" width="244" height="91" border="0" id="Image0'+(i+1)+'s" onmouseover="MM_swapImage(\'Image0'+(i+1)+'\',\'\',\'/oss/oss/common/images//banner/B_04_half_genre_b.png\',1);snd.playSE(cSE_Forcus);" onmouseout="MM_swapImgRestore();" onclick="snd.playSE(cSE_Decide);" /></a></div>';
  }
 }
 box.innerHTML=html;
 if(typeof setUnderButtonL=='function')setUnderButtonL(true,'Back','javascript:showBack()','snd.playSE(cSE_Cancel)');
}
window.initPage=function(){render();};
if(typeof window.kdown!='function')window.kdown=function(){var e=window.event,n=document.getElementById('searchItem');if(!e||!n)return;if(e.keyCode==38)n.scrollTop-=30;else if(e.keyCode==40)n.scrollTop+=30;};
if(window.setTimeout)setTimeout(render,700);else render();
})();`)
}
