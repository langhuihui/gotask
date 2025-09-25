import React from "react";

interface LogoProps {
  size?: "small" | "medium" | "large";
  showText?: boolean;
  className?: string;
}

const Logo: React.FC<LogoProps> = ({
  size = "medium",
  showText = true,
  className = "",
}) => {
  const sizeMap = {
    small: { width: 24, height: 24 },
    medium: { width: 32, height: 32 },
    large: { width: 48, height: 48 },
  };

  const { width, height } = sizeMap[size];

  return (
    <div className={`flex items-center gap-2 ${className}`}>
      <svg
        width={width}
        height={height}
        viewBox="0 0 32 32"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* 背景圆形 */}
        <circle
          cx="16"
          cy="16"
          r="15"
          fill="#1890ff"
          stroke="#40a9ff"
          stroke-width="1"
        />

        {/* 任务图标 - 简化的齿轮 */}
        <g transform="translate(8, 8)">
          {/* 外齿轮 */}
          <circle
            cx="8"
            cy="8"
            r="6"
            fill="none"
            stroke="white"
            stroke-width="1.5"
          />
          {/* 齿轮齿 */}
          <rect x="7" y="2" width="2" height="1.5" fill="white" />
          <rect x="7" y="12.5" width="2" height="1.5" fill="white" />
          <rect x="2" y="7" width="1.5" height="2" fill="white" />
          <rect x="12.5" y="7" width="1.5" height="2" fill="white" />
          {/* 内圆 */}
          <circle cx="8" cy="8" r="3" fill="white" />
          {/* 中心点 */}
          <circle cx="8" cy="8" r="1" fill="#1890ff" />
        </g>
      </svg>

      {showText && <span className="font-bold text-blue-600">GoTask</span>}
    </div>
  );
};

export default Logo;
