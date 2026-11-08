/* Mario download scene, as in the real Wii Shop Channel. The scene is chosen by
 * the documented shop.setSCARank value (OSC's Wii Shop Channel docs):
 *   0  small/big Mario collecting coins (the everyday scene)
 *   1  Star Mario (invincible, gold and white flicker)
 *   2  Star Mario running through Bullet Bills (Gold Ambassador)
 *   3  Fire Mario, white cap and red overalls; A shoots fireballs
 * Sprites from the permissioned wiishopchannel.net recreation, plus matching
 * Bullet Bill, star/fire Mario and fireball artwork. DOM sprites keep the scene
 * working in the Wii's Opera browser. Progress is supplied by Shroombrew; the
 * animation never invents completion.
 */
function ShroomMarioDownload(host, rank) {
 var self=this,base='/archive-shop/assets/anim/',tick=0,target=0,stage=0,x=-32,jump=0,hit=false,ending=0,callback=null,stopped=false;
 var RANK=(rank>=0&&rank<=3)?Math.floor(rank):0;
 var STAR=RANK===1||RANK===2;
 var FIRE=RANK===3;
 var scene=document.createElement('div');scene.id='shroomMarioScene';
 scene.style.cssText='position:absolute;left:44px;top:170px;width:520px;height:132px;overflow:hidden;z-index:60';
 scene.setAttribute('aria-label','Mario download animation');host.appendChild(scene);
 function sprite(name,w,h){var img=document.createElement('img');img.src=base+name+'.png';img.alt='';img.width=w;img.height=h;img.style.cssText='position:absolute;image-rendering:pixelated';scene.appendChild(img);return img;}
 function place(img,a,b){img.style.left=Math.round(a)+'px';img.style.top=Math.round(b)+'px';}
 var positions=[76,244,412],boxes=[],shadows=[];
 for(var i=0;i<3;i++){shadows[i]=sprite('shadowbox',40,40);place(shadows[i],positions[i]-8,18);boxes[i]=sprite('box',32,32);place(boxes[i],positions[i],18);}
 var shadow=sprite('shadow',32,8),mario=sprite('mario1',32,32),reward=sprite('boxcoin1',16,24);
 var coins=[],fireballs=[];
 reward.style.display='none';var rewardFrame=0,rewardX=0;
 var lastCoin=0,collectSound=null,jumpSound=null,fireSound=null;
 try{if(typeof Audio!=='undefined'){collectSound=new Audio('/archive-shop/assets/sounds/MARIO_COIN.WAV');jumpSound=new Audio('/archive-shop/assets/sounds/MARIO_JUMPL.WAV');fireSound=new Audio('/archive-shop/assets/sounds/MARIO_FIRE.WAV');}}catch(e){}
 function play(a){try{if(a){a.currentTime=0;var p=a.play();if(p&&p['catch'])p['catch'](function(){});}}catch(e){}}
 // On the console the shop's own sound engine is loaded, so use its effects
 // (9 small jump, 11 fireball, 12 coin) instead of the browser Audio fallback.
 function se(code,a){if(typeof shroomNativeSound!=='undefined'&&shroomNativeSound&&typeof snd!=='undefined'&&snd&&snd.playSE){try{snd.playSE(code);return;}catch(e){}}play(a);}
 // What the day's scene drops from the right edge. Rank 2 is the Gold
 // Ambassador scene: Star Mario and Bullet Bills.
 function drop(){
  if(RANK===2){var b=sprite('bullet1',32,24);place(b,520,101);coins.push({node:b,x:520,step:8,kind:'bullet'});}
  else{var c=sprite('coin1',12,21);place(c,520,105);coins.push({node:c,x:520,step:8,kind:'coin'});}
 }
 // Star Mario blinks between the gold and white palettes. Fire Mario keeps his
 // white cap and red overalls.
 function marioFrame(n){
  if(FIRE)return 'mariofire'+n;
  if(!STAR)return 'mario'+n;
  return (Math.floor(tick/3)%2?'mariostarb':'mariostar')+n;
 }
 function shoot(){
  if(!FIRE||stopped)return;
  var f=sprite('fireball1',10,10);place(f,x+26,100);
  fireballs.push({node:f,x:x+26,step:9});
  se(11,fireSound);
 }
 function onKey(e){e=e||window.event;var k=e?e.keyCode:0;if(k===13||k===65)shoot();}
 try{if(document.addEventListener)document.addEventListener('keydown',onKey,false);else document.onkeydown=onKey;}catch(e){}
 self.setProgress=function(value){target=Math.max(target,Math.min(1,Math.max(0,value)));};
 self.stop=function(){if(stopped)return;stopped=true;clearInterval(timer);try{if(document.removeEventListener)document.removeEventListener('keydown',onKey,false);else document.onkeydown=null;}catch(e){}if(scene.parentNode)scene.parentNode.removeChild(scene);};
 self.finish=function(done){target=1;callback=done;};
 var timer=setInterval(function(){
  tick++;var y=100,frame=(Math.floor(tick/4)%3)+2;
  if(lastCoin<Math.floor(target*100)&&tick%3===0){drop();lastCoin++;}
  for(var c=coins.length-1;c>=0;c--){
   var v=coins[c];v.x-=v.step;
   if(v.kind==='bullet')v.node.src=base+'bullet'+((Math.floor(tick/3)%4)+1)+'.png';
   else v.node.src=base+'coin'+((Math.floor(tick/3)%4)+1)+'.png';
   place(v.node,v.x,v.kind==='bullet'?101:105);
   if(v.x<x+22){scene.removeChild(v.node);coins.splice(c,1);if(tick%5===0)se(12,collectSound);}
  }
  for(var f=fireballs.length-1;f>=0;f--){
   var fb=fireballs[f];fb.x+=fb.step;
   fb.node.src=base+'fireball'+((Math.floor(tick/4)%2)+1)+'.png';place(fb.node,fb.x,100);
   if(fb.x>540){scene.removeChild(fb.node);fireballs.splice(f,1);}
  }
  var threshold=stage===0?.33:stage===1?.66:.99;
  if(jump){
   jump++;y=100-52*Math.sin(Math.PI*jump/24);frame=5;
   if(jump===12&&!hit){hit=true;se(12,collectSound);rewardX=positions[stage]+8;rewardFrame=1;reward.style.display='';if(stage<3)boxes[stage].src=base+'boxused.png';}
   if(jump>24){jump=0;hit=false;if(stage<3)stage++;if(stage===3)stage=4;}
  }else if(stage<3&&target>=threshold){
   var dest=positions[stage];
   if(Math.abs(x-dest)<5){x=dest;jump=1;se(9,jumpSound);}else x+=x<dest?5:-5;
  }else if(stage<3){x+=4;if(x>550)x=-32;}
  else if(stage===4){
   // The third block is the multi-coin block: keep hitting it from 99% until
   // the shop reports the download finished.
   x=positions[2];
   if(!jump){jump=1;se(9,jumpSound);}
   if(target>=1)ending=1;
  }
  if(rewardFrame){rewardFrame++;place(reward,rewardX,6-40*Math.sin(Math.PI*Math.min(30,rewardFrame)/30));reward.src=base+'boxcoin'+((Math.floor(tick/3)%4)+1)+'.png';if(rewardFrame>30){reward.style.display='none';rewardFrame=0;}}
  if(ending){ending++;if(ending>24&&callback){var done=callback;callback=null;self.stop();done();return;}}
  mario.src=base+marioFrame(frame)+'.png';place(mario,x,y);place(shadow,x,128);
 },33);
}
