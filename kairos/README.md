<div align="center">
    <br>
    <br>
    <img width="182" src="../.github/assets/k2.png">
    <h1 align="center">K2</h1>
</div>

<p align="center">
<b>Kairos Cloud Config</b>
</p>

<hr>
<br>
<br>

## Usage

1. Spin up a Kairos node
2. Inject secrets and copy the config to clipboard:
```
$ op inject -i <file> | pbcopy
```
3. Use the copied config as the cloud config of the node

## Templates
| File Name        | Description                                 |
| ---------------- | ------------------------------------------- |
| `bootstrap.yaml` | Config for setting up the first master node |
| `master.yaml`    | Second master node and onwards              |
| `worder.yaml`    | Worker nodes                                |
