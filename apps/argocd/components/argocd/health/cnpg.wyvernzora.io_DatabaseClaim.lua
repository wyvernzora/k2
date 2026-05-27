hs = { status = "Progressing", message = "Waiting for DatabaseClaim status" }

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      hs.message = condition.message or condition.reason or ""

      if condition.status == "True" then
        hs.status = "Healthy"
        return hs
      end

      if condition.status == "False" then
        if condition.reason == "ClusterNotReady" or condition.reason == "Reconciling" then
          hs.status = "Progressing"
        else
          hs.status = "Degraded"
        end
        return hs
      end
    end
  end
end

return hs
