with recursive cte (idx, payer, receiver) as (                      
    select 1 as idx, payer, receiver from payment where payer = 'user00000716'
    union
    select t1.idx+1 as idx, t2.payer, t2.receiver from payment t2
    inner join cte t1
    on t1.receiver = t2.payer
    and t1.idx < {{ .RecursiveNum }}
) select count(*) from cte
