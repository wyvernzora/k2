import { EnvValue, type ISecret } from "cdk8s-plus-32";

import { FORGEJO_HOST, FORGEJO_HTTP_PORT, FORGEJO_SSH_PORT } from "../../constants.js";

export interface ForgejoEnvProps {
  readonly credentialsSecret: ISecret;
  readonly forgejoSecret: ISecret;
}

export function forgejoEnv(props: ForgejoEnvProps): Record<string, EnvValue> {
  return {
    DB_HOST: props.credentialsSecret.envValue("host"),
    DB_PORT: props.credentialsSecret.envValue("port"),
    DB_NAME: props.credentialsSecret.envValue("dbname"),
    DB_USER: props.credentialsSecret.envValue("user"),
    DB_PASSWORD: props.credentialsSecret.envValue("password"),
    USER_UID: EnvValue.fromValue("1000"),
    USER_GID: EnvValue.fromValue("1000"),
    FORGEJO_WORK_DIR: EnvValue.fromValue("/var/lib/gitea"),
    FORGEJO__database__DB_TYPE: EnvValue.fromValue("postgres"),
    FORGEJO__database__HOST: EnvValue.fromValue("$(DB_HOST):$(DB_PORT)"),
    FORGEJO__database__NAME: EnvValue.fromValue("$(DB_NAME)"),
    FORGEJO__database__USER: EnvValue.fromValue("$(DB_USER)"),
    FORGEJO__database__PASSWD: EnvValue.fromValue("$(DB_PASSWORD)"),
    FORGEJO__database__SSL_MODE: EnvValue.fromValue("disable"),
    FORGEJO__security__INSTALL_LOCK: EnvValue.fromValue("true"),
    FORGEJO__security__SECRET_KEY: props.forgejoSecret.envValue("secretKey"),
    FORGEJO__security__INTERNAL_TOKEN: props.forgejoSecret.envValue("internalToken"),
    FORGEJO__server__APP_DATA_PATH: EnvValue.fromValue("/var/lib/gitea/data"),
    FORGEJO__server__DOMAIN: EnvValue.fromValue(FORGEJO_HOST),
    FORGEJO__server__ROOT_URL: EnvValue.fromValue(`https://${FORGEJO_HOST}/`),
    FORGEJO__server__HTTP_ADDR: EnvValue.fromValue("0.0.0.0"),
    FORGEJO__server__HTTP_PORT: EnvValue.fromValue(String(FORGEJO_HTTP_PORT)),
    FORGEJO__server__DISABLE_SSH: EnvValue.fromValue("false"),
    FORGEJO__server__START_SSH_SERVER: EnvValue.fromValue("true"),
    FORGEJO__server__SSH_DOMAIN: EnvValue.fromValue(FORGEJO_HOST),
    FORGEJO__server__SSH_PORT: EnvValue.fromValue(String(FORGEJO_SSH_PORT)),
    FORGEJO__server__SSH_LISTEN_PORT: EnvValue.fromValue(String(FORGEJO_SSH_PORT)),
    FORGEJO__service__DISABLE_REGISTRATION: EnvValue.fromValue("true"),
    FORGEJO__service__REQUIRE_SIGNIN_VIEW: EnvValue.fromValue("true"),
    FORGEJO__openid__ENABLE_OPENID_SIGNIN: EnvValue.fromValue("false"),
    FORGEJO__openid__ENABLE_OPENID_SIGNUP: EnvValue.fromValue("false"),
    FORGEJO__oauth2_client__ENABLE_AUTO_REGISTRATION: EnvValue.fromValue("true"),
    FORGEJO__oauth2_client__USERNAME: EnvValue.fromValue("nickname"),
    FORGEJO__oauth2_client__ACCOUNT_LINKING: EnvValue.fromValue("login"),
    FORGEJO__repository__DEFAULT_PRIVATE: EnvValue.fromValue("private"),
    FORGEJO__repository__DEFAULT_PUSH_CREATE_PRIVATE: EnvValue.fromValue("true"),
  };
}
