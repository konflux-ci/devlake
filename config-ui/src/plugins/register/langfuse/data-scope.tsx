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

import { useMemo, useState } from 'react';
import { Button, Flex, Input, Table, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';

interface ScopeItem {
  id: string;
  name: string;
  fullName: string;
  data?: Record<string, unknown>;
}

interface Props {
  connectionId: ID;
  disabledItems?: Array<{ id: ID }>;
  selectedItems: ScopeItem[];
  onChangeSelectedItems: (items: ScopeItem[]) => void;
}

export const LangfuseDataScope = ({
  connectionId: _connectionId,
  disabledItems,
  selectedItems,
  onChangeSelectedItems,
}: Props) => {
  const [projectId, setProjectId] = useState('');

  const disabledIds = useMemo(() => new Set(disabledItems?.map((it) => String(it.id)) ?? []), [disabledItems]);
  const existingIds = useMemo(() => new Set(selectedItems.map((it) => it.id)), [selectedItems]);

  const handleAdd = () => {
    const trimmed = projectId.trim();
    if (!trimmed || existingIds.has(trimmed) || disabledIds.has(trimmed)) {
      return;
    }

    const item: ScopeItem = {
      id: trimmed,
      name: trimmed,
      fullName: trimmed,
      data: { projectId: trimmed, name: trimmed },
    };

    onChangeSelectedItems([...selectedItems, item]);
    setProjectId('');
  };

  const handleRemove = (id: string) => {
    onChangeSelectedItems(selectedItems.filter((item) => item.id !== id));
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAdd();
    }
  };

  const columns: ColumnsType<ScopeItem> = [
    {
      title: 'Project',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '',
      dataIndex: 'id',
      key: 'action',
      width: 80,
      align: 'center',
      render: (id: string) => (
        <Tooltip title={disabledIds.has(id) ? 'Scope is used by existing blueprint' : 'Remove'}>
          <Button
            type="text"
            danger
            icon={<DeleteOutlined />}
            disabled={disabledIds.has(id)}
            onClick={() => handleRemove(id)}
          />
        </Tooltip>
      ),
    },
  ];

  const canAdd = projectId.trim() && !existingIds.has(projectId.trim()) && !disabledIds.has(projectId.trim());

  return (
    <Flex vertical gap="middle">
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        Enter a project identifier for your Langfuse project. This is a label you choose — each Langfuse API key pair
        corresponds to one project.
      </Typography.Paragraph>

      <Flex gap="small">
        <Input
          style={{ flex: 1 }}
          placeholder="e.g. dvw-agents-telemetry"
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <Button type="primary" icon={<PlusOutlined />} disabled={!canAdd} onClick={handleAdd}>
          Add
        </Button>
      </Flex>

      <Table
        size="middle"
        rowKey="id"
        columns={columns}
        dataSource={selectedItems}
        pagination={false}
        locale={{ emptyText: 'No project added yet.' }}
      />
    </Flex>
  );
};
