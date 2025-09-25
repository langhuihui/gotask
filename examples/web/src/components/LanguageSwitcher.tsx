import React from "react";
import { Button, Dropdown, Space } from "antd";
import { GlobalOutlined } from "@ant-design/icons";
import type { MenuProps } from "antd";
import { useLanguage } from "../hooks/useLanguage";

const LanguageSwitcher: React.FC = () => {
  const { t, changeLanguage, currentLanguage } = useLanguage();

  const items: MenuProps["items"] = [
    {
      key: "zh",
      label: t("language.chinese"),
    },
    {
      key: "en",
      label: t("language.english"),
    },
  ];

  const handleMenuClick: MenuProps["onClick"] = ({ key }) => {
    changeLanguage(key as 'zh' | 'en');
  };

  return (
    <Dropdown
      menu={{ items, onClick: handleMenuClick }}
      placement="bottomRight"
      trigger={["click"]}
    >
      <Button icon={<GlobalOutlined />}>
        <Space>
          {currentLanguage === "zh"
            ? t("language.chinese")
            : t("language.english")}
        </Space>
      </Button>
    </Dropdown>
  );
};

export default LanguageSwitcher;
