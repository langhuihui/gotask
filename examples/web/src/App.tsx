import React from "react";
import { Layout, Typography } from "antd";
import TaskTree from "./components/TaskTree";

const { Header, Content } = Layout;
const { Title } = Typography;

const App: React.FC = () => {
  return (
    <Layout style={{ minHeight: "100vh", width: "100%", maxWidth: "none" }}>
      <Header
        style={{
          background: "#fff",
          padding: "0 24px",
          boxShadow: "0 1px 4px rgba(0,0,0,0.1)",
        }}
      >
        <Title level={3} style={{ margin: "16px 0", color: "#1890ff" }}>
          GoTask 任务管理系统
        </Title>
      </Header>

      <Content
        style={{
          padding: "24px",
          background: "#f0f2f5",
          width: "100%",
          maxWidth: "none",
        }}
      >
        <TaskTree />
      </Content>
    </Layout>
  );
};

export default App;
