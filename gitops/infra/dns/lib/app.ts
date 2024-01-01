import { App, YamlOutputType } from 'cdk8s'
import { BlockyApp } from './blocky'
import { UnboundApp } from './unbound';

const app = new App({
    yamlOutputType: YamlOutputType.FILE_PER_APP,
});

const unbound = new UnboundApp(app, 'unbound');

new BlockyApp(app, 'blocky-app', {
    upstreams: [unbound.service.name],
    blockLists: [
        'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts'
    ],
});

app.synth();
