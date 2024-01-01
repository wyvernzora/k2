import { App } from 'cdk8s'
import { BlockyApp } from './blocky'

const app = new App();

new BlockyApp(app, 'blocky-app', {
    upstreams: ['one.one.one.one'],
    blockLists: [
        'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts'
    ],
});

app.synth();
