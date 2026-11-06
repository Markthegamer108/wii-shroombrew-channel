/* Mario/blocks/coins from the permissioned wiishopchannel.net recreation.
 * DOM sprites keep the scene compatible with the Wii's Opera browser.
 * Progress is supplied by Shroombrew; the animation never completes a job.
 */
function ShroomMarioDownload(host) {
 var self=this,base='/archive-shop/assets/anim/',tick=0,target=0,stage=0,x=-32,jump=0,hit=false,ending=0,callback=null,stopped=false;
 var scene=document.createElement('div');scene.id='shroomMarioScene';
 scene.style.cssText='position:absolute;left:44px;top:170px;width:520px;height:132px;overflow:hidden;z-index:60';
 scene.setAttribute('aria-label','Mario download animation');host.appendChild(scene);
 function sprite(name,w,h){var img=document.createElement('img');img.src=base+name+'.png';img.alt='';img.width=w;img.height=h;img.style.cssText='position:absolute;image-rendering:pixelated';scene.appendChild(img);return img;}
 function place(img,a,b){img.style.left=Math.round(a)+'px';img.style.top=Math.round(b)+'px';}
 var positions=[76,244,412],boxes=[],shadows=[];
 for(var i=0;i<3;i++){shadows[i]=sprite('shadowbox',40,40);place(shadows[i],positions[i]-8,18);boxes[i]=sprite('box',32,32);place(boxes[i],positions[i],18);}
 var shadow=sprite('shadow',32,8),mario=sprite('mario1',32,32),reward=sprite('boxcoin1',16,24),coins=[];
 reward.style.display='none';var rewardFrame=0,rewardX=0;
 var lastCoin=0,collectSound=null,jumpSound=null;
 try{if(typeof Audio!=='undefined'){collectSound=new Audio('/archive-shop/assets/sounds/MARIO_COIN.WAV');jumpSound=new Audio('/archive-shop/assets/sounds/MARIO_JUMPL.WAV');}}catch(e){}
 function play(a){try{if(a){a.currentTime=0;var p=a.play();if(p&&p['catch'])p['catch'](function(){});}}catch(e){}}
 // On the console the shop's own sound engine is loaded, so use its coin and
 // jump effects instead of the browser Audio fallback.
 function se(code,a){if(typeof shroomNativeSound!=='undefined'&&shroomNativeSound&&typeof snd!=='undefined'&&snd&&snd.playSE){try{snd.playSE(code);return;}catch(e){}}play(a);}
 self.setProgress=function(value){target=Math.max(target,Math.min(1,Math.max(0,value)));};
 self.stop=function(){if(stopped)return;stopped=true;clearInterval(timer);if(scene.parentNode)scene.parentNode.removeChild(scene);};
 self.finish=function(done){target=1;callback=done;};
 var timer=setInterval(function(){
  tick++;var y=100,frame=(Math.floor(tick/4)%3)+2;
  if(lastCoin<Math.floor(target*100)&&tick%3===0){var coin=sprite('coin1',12,21);place(coin,520,105);coins.push({node:coin,x:520});lastCoin++;}
  for(var c=coins.length-1;c>=0;c--){var v=coins[c];v.x-=8;v.node.src=base+'coin'+((Math.floor(tick/3)%4)+1)+'.png';place(v.node,v.x,105);if(v.x<x+22){scene.removeChild(v.node);coins.splice(c,1);if(tick%5===0)se(12,collectSound);}}
  var threshold=stage===0 ? .33 : stage===1 ? .66 : .99;
  if(jump){
   jump++;y=100-52*Math.sin(Math.PI*jump/24);frame=5;
   if(jump===12&&!hit){hit=true;se(12,collectSound);rewardX=positions[stage]+8;rewardFrame=1;reward.style.display='';if(stage<2)boxes[stage].src=base+'boxused.png';}
   if(jump>24){jump=0;hit=false;stage++;if(stage===3)ending=1;}
  }else if(stage<3&&target>=threshold){
   var dest=positions[stage];
   if(Math.abs(x-dest)<5){x=dest;jump=1;se(9,jumpSound);}else x+=x<dest?5:-5;
  }else if(stage<3){x+=4;if(x>550)x=-32;}
  if(rewardFrame){rewardFrame++;place(reward,rewardX,6-40*Math.sin(Math.PI*Math.min(30,rewardFrame)/30));reward.src=base+'boxcoin'+((Math.floor(tick/3)%4)+1)+'.png';if(rewardFrame>30){reward.style.display='none';rewardFrame=0;}}
  if(ending){ending++;frame=1;if(ending>24&&callback){var done=callback;callback=null;self.stop();done();return;}}
  mario.src=base+'mario'+frame+'.png';place(mario,x,y);place(shadow,x,128);
 },33);
}
