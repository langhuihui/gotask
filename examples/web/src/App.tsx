import React, { useEffect, useState } from "react";
import { Layout, Typography, Tabs } from "antd";
import TaskTree from "./components/TaskTree";
import TaskHistoryView from "./components/TaskHistoryView";
import LanguageSwitcher from "./components/LanguageSwitcher";
import { useLanguage } from "./hooks/useLanguage";

const { Header, Content } = Layout;
const { Title } = Typography;

const App: React.FC = () => {
  const { t, currentLanguage } = useLanguage();
  const [activeTab, setActiveTab] = useState("task-tree");

  // 更新页面标题
  useEffect(() => {
    document.title = t("app.title");
  }, [t, currentLanguage]);

  const tabItems = [
    {
      key: "task-tree",
      label: t("task.tree"),
      children: <TaskTree />,
    },
    {
      key: "task-history",
      label: t("history.title"),
      children: <TaskHistoryView />,
    },
  ];

  return (
    <Layout style={{ minHeight: "100vh", width: "100%", maxWidth: "none" }}>
      <Header
        style={{
          background: "#fff",
          padding: "0 24px",
          boxShadow: "0 1px 4px rgba(0,0,0,0.1)",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Title level={3} style={{ margin: "16px 0", color: "#1890ff" }}>
          {t("app.title")}
        </Title>
        <LanguageSwitcher />
      </Header>

      <Content
        style={{
          padding: "24px",
          background: "#f0f2f5",
          width: "100%",
          maxWidth: "none",
        }}
      >
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={tabItems}
          size="large"
        />
      </Content>
    </Layout>
  );
};

export default App;
