import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
const root = new URL('../../../', import.meta.url);
registerHooks({resolve(specifier, context, next) {
 if (specifier.startsWith('@/')) {
  const path = new URL(specifier.slice(2)+'.ts',root);
  if (existsSync(fileURLToPath(path))) return next(path.href,context);
 }
 return next(specifier,context);
}});
const {extractArtifactsFromContent:single,extractArtifactsFromMessages:extract,resolveConversationArtifact:resolve} = await import('./chat-artifacts.ts');
const msg=(id,code)=>({publicID:id,key:id,runID:'run_'+id,role:'assistant',content:'```html\n'+code});
test('every segment opens the same complete continuation with stable identity',()=>{
 const messages=[msg('a','<!doctype html><html><body>first'),msg('b','middle'),msg('c','last</body></html>\n```')];
 const artifacts=extract(messages);
 assert.equal(artifacts.length,1);
 assert.equal(artifacts[0].id,single(messages[0])[0].id);
 for(const message of messages)assert.equal(resolve(artifacts,single(message)[0]),artifacts[0]);
 assert.match(artifacts[0].code,/first\nmiddle\nlast/);
});
test('new document and later blocks retain their own identity after a continuation',()=>{
 const messages=[msg('a','<!doctype html><html><body>first'),msg('b','end</body></html>\n```\n```html\n<!doctype html><html>new</html>\n```')];
 const artifacts=extract(messages),sources=single(messages[1]);
 assert.equal(artifacts.length,2);
 assert.equal(resolve(artifacts,sources[0]),artifacts[0]);
 assert.equal(resolve(artifacts,sources[1]),artifacts[1]);
});
test('identical code in unrelated messages does not select another artifact',()=>{
 const messages=[msg('a','<html>same</html>\n```'),msg('b','<html>same</html>\n```')];
 const artifacts=extract(messages);
 assert.equal(resolve(artifacts,single(messages[1])[0]),artifacts[1]);
});
