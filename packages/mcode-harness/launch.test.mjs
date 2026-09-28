import assert from 'node:assert/strict';
import test from 'node:test';
import {mkdtemp,mkdir,writeFile,copyFile,rm,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {ToolExecutor} from './tool-executor.mjs';

test('Runtime directly executes host worker with bound cwd and initialized environment',async t=>{
 const root=await realpath(await mkdtemp(join(tmpdir(),'mcode-host-')));
 t.after(()=>rm(root,{recursive:true,force:true}));
 await mkdir(join(root,'dist'));
 const workspace=join(root,'workspace');await mkdir(workspace);
 const launcher=join(root,'launch.mjs');await copyFile(new URL('./launch.mjs',import.meta.url),launcher);
 await writeFile(join(root,'dist','worker.mjs'),`import {readFileSync} from 'node:fs';const request=JSON.parse(readFileSync(0,'utf8'));console.log(JSON.stringify({tool_name:request.tool,text:JSON.stringify({cwd:process.cwd(),value:process.env.INITIALIZED}),content:[]}));`);
 const profile=join(root,'profile.json');
 await writeFile(profile,JSON.stringify({workspace,scratch:join(root,'scratch'),network:'enabled',toolEnv:{INITIALIZED:'ordinary'}}));
 const executor=new ToolExecutor(profile,launcher);t.after(()=>executor.close());
 const result=await executor.execute('bash',{});
 assert.deepEqual(JSON.parse(result.text),{cwd:workspace,value:'ordinary'});
});
