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

import { useEffect, useState } from 'react';
import { Divider, Form, Input, message, Modal, Select, Spin } from 'antd';

import API from '@/api';
import { IAgentReadyScopeConfig } from '@/api/plugin/agentready';
import { IConnectionAPI } from '@/types';

const DEFAULT_ASSESSMENT_FILE_PATH = '.agentready/assessment-latest.json';
const DEFAULT_SUBMISSIONS_PATH = 'submissions';

interface Props {
  scopeConfigId?: number;
  onCancel: () => void;
  onSave: (id: number) => void;
}

export const AgentReadyScopeConfigModal = ({ scopeConfigId, onCancel, onSave }: Props) => {
  const [form] = Form.useForm<IAgentReadyScopeConfig>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [connections, setConnections] = useState<IConnectionAPI[]>([]);

  useEffect(() => {
    API.connection
      .list('github')
      .then(setConnections)
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (!scopeConfigId) {
      form.setFieldsValue({
        assessmentFilePath: DEFAULT_ASSESSMENT_FILE_PATH,
        submissionsPath: DEFAULT_SUBMISSIONS_PATH,
      });
      return;
    }

    const load = async () => {
      setLoading(true);
      try {
        const config = await API.plugin.agentready.getScopeConfig(scopeConfigId);
        if (config) {
          form.setFieldsValue(config);
        }
      } catch {
        message.error('Failed to load configuration.');
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [scopeConfigId, form]);

  const handleSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const saved = scopeConfigId
        ? await API.plugin.agentready.updateScopeConfig(scopeConfigId, values)
        : await API.plugin.agentready.createScopeConfig(values);
      if (saved.id == null) {
        message.error('Saved configuration is missing an ID.');
        return;
      }
      onSave(saved.id);
    } catch {
      message.error('Failed to save configuration. Please try again.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open
      width={600}
      title="Agent Ready Configuration"
      okText="Save"
      confirmLoading={saving}
      onCancel={onCancel}
      onOk={handleSave}
    >
      <Spin spinning={loading}>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            label="Branch"
            name="branch"
            tooltip="Git branch to read assessments from. Leave empty to use the repository's default branch."
          >
            <Input placeholder="e.g. main" />
          </Form.Item>
          <Form.Item
            label="Assessment File Path"
            name="assessmentFilePath"
            tooltip="Path to the assessment JSON file within each repository."
          >
            <Input placeholder={DEFAULT_ASSESSMENT_FILE_PATH} />
          </Form.Item>
          <Form.Item
            label="Exclude Repos"
            name="excludeRepos"
            tooltip="Comma-separated list of repository names to exclude from assessment collection."
          >
            <Input.TextArea rows={3} placeholder="e.g. repo-a, repo-b" />
          </Form.Item>
          <Divider orientation="left" plain>
            Submissions Repository (Optional)
          </Divider>
          <Form.Item
            label="Submissions Repo"
            name="submissionsRepo"
            tooltip="GitHub full name of the centralized submissions repository (e.g. org/repo). Leave empty to disable bulk onboarding."
          >
            <Input placeholder="e.g. ambient-code/agentready" />
          </Form.Item>
          <Form.Item
            label="Submissions Path"
            name="submissionsPath"
            tooltip="Directory within the submissions repo containing {org}/{repo}/{file}.json structure."
          >
            <Input placeholder={DEFAULT_SUBMISSIONS_PATH} />
          </Form.Item>
          <Form.Item
            label="GitHub Connection"
            name="submissionsConnectionId"
            tooltip="GitHub connection used to authenticate API calls to the submissions repository."
          >
            <Select
              allowClear
              placeholder="Select a GitHub connection"
              options={connections.map((c) => ({ label: c.name, value: c.id }))}
            />
          </Form.Item>
        </Form>
      </Spin>
    </Modal>
  );
};
