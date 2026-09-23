/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

import { useState, useEffect } from 'react';
import type { RadioChangeEvent } from 'antd';
import { Radio, Input } from 'antd';

import { Block, ExternalLink } from '@/components';
import { DOC_URL } from '@/release';

const JIRA_CLOUD_REGEX = /^https:\/\/\w+.atlassian.net\/rest\/$/;
const JIRA_OAUTH_GATEWAY_REGEX = /^https:\/\/api\.atlassian\.com\/ex\/jira\/[^/]+\/rest\/$/;

type Method = 'BasicAuth' | 'AccessToken' | 'OAuth2';
type CloudMethod = 'BasicAuth' | 'OAuth2';

const gatewayEndpoint = (cloudId?: string) =>
  cloudId ? `https://api.atlassian.com/ex/jira/${cloudId.trim()}/rest/` : '';

const isCloudConnection = (values: { authMethod?: string; endpoint?: string }) =>
  values.authMethod === 'OAuth2' ||
  !values.endpoint ||
  JIRA_CLOUD_REGEX.test(values.endpoint) ||
  JIRA_OAUTH_GATEWAY_REGEX.test(values.endpoint);

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
  errors: any;
  setValues: (value: any) => void;
  setErrors: (value: any) => void;
}

export const Auth = ({ type, initialValues, values, setValues, setErrors }: Props) => {
  const [version, setVersion] = useState('cloud');

  useEffect(() => {
    if (isCloudConnection(initialValues)) {
      setVersion('cloud');
    } else if (initialValues.endpoint) {
      setVersion('server');
    }
  }, [initialValues.endpoint, initialValues.authMethod]);

  useEffect(() => {
    setValues({
      endpoint: initialValues.endpoint,
      authMethod: initialValues.authMethod ?? 'BasicAuth',
      username: initialValues.username,
      password: initialValues.password,
      token: initialValues.token,
      cloudId: initialValues.cloudId,
      clientId: initialValues.clientId,
      clientSecret: initialValues.clientSecret,
    });
  }, [
    initialValues.endpoint,
    initialValues.authMethod,
    initialValues.username,
    initialValues.password,
    initialValues.token,
    initialValues.cloudId,
    initialValues.clientId,
    initialValues.clientSecret,
  ]);

  useEffect(() => {
    const required =
      (values.authMethod === 'BasicAuth' && values.username && values.password) ||
      (values.authMethod === 'AccessToken' && values.token) ||
      (values.authMethod === 'OAuth2' && values.cloudId && values.clientId && values.clientSecret) ||
      type === 'update';
    setErrors({
      endpoint: values.authMethod === 'OAuth2' || values.endpoint ? '' : 'endpoint is required',
      auth: required ? '' : 'auth is required',
    });
  }, [values]);

  const handleChangeVersion = (e: RadioChangeEvent) => {
    const nextVersion = e.target.value;

    setValues({
      endpoint: '',
      authMethod: 'BasicAuth',
      username: undefined,
      password: undefined,
      token: undefined,
      cloudId: undefined,
      clientId: undefined,
      clientSecret: undefined,
    });

    setVersion(nextVersion);
  };

  const handleChangeEndpoint = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      endpoint: e.target.value,
    });
  };

  const handleChangeCloudMethod = (e: RadioChangeEvent) => {
    const authMethod = (e.target as HTMLInputElement).value as CloudMethod;
    setValues({
      authMethod,
      username: undefined,
      password: undefined,
      token: undefined,
      cloudId: undefined,
      clientId: undefined,
      clientSecret: undefined,
      endpoint: '',
    });
  };

  const handleChangeMethod = (e: RadioChangeEvent) => {
    setValues({
      authMethod: (e.target as HTMLInputElement).value as Method,
      username: undefined,
      password: undefined,
      token: undefined,
    });
  };

  const handleChangeUsername = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      username: e.target.value,
    });
  };

  const handleChangePassword = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      password: e.target.value,
    });
  };

  const handleChangeToken = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      token: e.target.value,
    });
  };

  const handleChangeCloudId = (e: React.ChangeEvent<HTMLInputElement>) => {
    const cloudId = e.target.value;
    setValues({
      cloudId,
      endpoint: gatewayEndpoint(cloudId),
    });
  };

  const handleChangeClientId = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      clientId: e.target.value,
    });
  };

  const handleChangeClientSecret = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValues({
      clientSecret: e.target.value,
    });
  };

  const cloudAuthMethod: CloudMethod = values.authMethod === 'OAuth2' ? 'OAuth2' : 'BasicAuth';

  return (
    <>
      <Block title="Jira Version" required>
        <Radio.Group value={version} onChange={handleChangeVersion}>
          <Radio value="cloud">Jira Cloud</Radio>
          <Radio value="server">Jira Server</Radio>
        </Radio.Group>

        {!(version === 'cloud' && cloudAuthMethod === 'OAuth2') && (
          <Block
            style={{ marginTop: 8, marginBottom: 0 }}
            title="Endpoint URL"
            description={
              <>
                {version === 'cloud'
                  ? 'Provide the Jira instance API endpoint. For Jira Cloud, e.g. https://your-company.atlassian.net/rest/. Please note that the endpoint URL should end with /.'
                  : ''}
                {version === 'server'
                  ? 'Provide the Jira instance API endpoint. For Jira Server, e.g. https://jira.your-company.com/rest/. Please note that the endpoint URL should end with /.'
                  : ''}
              </>
            }
            required
          >
            <Input
              style={{ width: 386 }}
              placeholder="Your Endpoint URL"
              value={values.endpoint}
              onChange={handleChangeEndpoint}
            />
          </Block>
        )}
      </Block>

      {version === 'cloud' && (
        <>
          <Block title="Authentication Method" required>
            <Radio.Group value={cloudAuthMethod} onChange={handleChangeCloudMethod}>
              <Radio value="BasicAuth">API Token</Radio>
              <Radio value="OAuth2">OAuth 2.0 (Service Account)</Radio>
            </Radio.Group>
          </Block>

          {cloudAuthMethod === 'BasicAuth' && (
            <>
              <Block title="E-Mail" required>
                <Input
                  style={{ width: 386 }}
                  placeholder="Your E-Mail"
                  value={values.username}
                  onChange={handleChangeUsername}
                />
              </Block>
              <Block
                title="API Token"
                description={
                  <ExternalLink link={DOC_URL.PLUGIN.JIRA.API_TOKEN}>
                    Learn about how to create an API Token
                  </ExternalLink>
                }
                required
              >
                <Input
                  style={{ width: 386 }}
                  placeholder={type === 'update' ? '********' : 'Your PAT'}
                  value={values.password}
                  onChange={handleChangePassword}
                />
              </Block>
            </>
          )}

          {cloudAuthMethod === 'OAuth2' && (
            <>
              <Block
                title="Cloud ID"
                description="The Atlassian Cloud ID for the Jira site. The API endpoint is derived as https://api.atlassian.com/ex/jira/{cloudId}/rest/."
                required
              >
                <Input
                  style={{ width: 386 }}
                  placeholder="Your Cloud ID"
                  value={values.cloudId}
                  onChange={handleChangeCloudId}
                />
              </Block>
              <Block title="Client ID" required>
                <Input
                  style={{ width: 386 }}
                  placeholder="OAuth 2.0 Client ID"
                  value={values.clientId}
                  onChange={handleChangeClientId}
                />
              </Block>
              <Block title="Client Secret" required>
                <Input.Password
                  style={{ width: 386 }}
                  placeholder={type === 'update' ? '********' : 'OAuth 2.0 Client Secret'}
                  value={values.clientSecret}
                  onChange={handleChangeClientSecret}
                />
              </Block>
              {values.endpoint && (
                <Block title="Endpoint URL" description="Derived from Cloud ID for Atlassian OAuth 2.0.">
                  <Input style={{ width: 386 }} value={values.endpoint} disabled />
                </Block>
              )}
            </>
          )}
        </>
      )}

      {version === 'server' && (
        <>
          <Block title="Authentication Method" required>
            <Radio.Group value={values.authMethod} onChange={handleChangeMethod}>
              <Radio value="BasicAuth">Basic Authentication</Radio>
              <Radio value="AccessToken">Using Personal Access Token</Radio>
            </Radio.Group>
          </Block>
          {values.authMethod === 'BasicAuth' && (
            <>
              <Block title="Username" required>
                <Input
                  style={{ width: 386 }}
                  placeholder="Your Username"
                  value={values.username}
                  onChange={handleChangeUsername}
                />
              </Block>
              <Block title="Password" required>
                <Input.Password
                  style={{ width: 386 }}
                  placeholder={type === 'update' ? '********' : 'Your Password'}
                  value={values.password}
                  onChange={handleChangePassword}
                />
              </Block>
            </>
          )}
          {values.authMethod === 'AccessToken' && (
            <Block
              title="Personal Access Token"
              description={
                <ExternalLink link={DOC_URL.PLUGIN.JIRA.PERSONAL_ACCESS_TOKEN}>
                  Learn about how to create a PAT
                </ExternalLink>
              }
              required
            >
              <Input.Password
                style={{ width: 386 }}
                placeholder={type === 'update' ? '********' : 'Your Password'}
                value={values.token}
                onChange={handleChangeToken}
              />
            </Block>
          )}
        </>
      )}
    </>
  );
};
